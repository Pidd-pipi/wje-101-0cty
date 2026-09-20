package service

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/dto"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/repository"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newBlindTestService(t *testing.T) (*BlindTastingService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:blind_%s?mode=memory&cache=shared&_busy_timeout=5000",
		strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Serialize access: SQLite has no FOR UPDATE; a single connection makes
	// the transaction-based state transitions deterministic like PG row locks.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{},
		&model.CoffeeBean{},
		&model.BlindTastingSession{},
		&model.BlindTastingParticipant{},
		&model.BlindScore{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewBlindTastingService(
		repository.NewBlindTastingRepository(db),
		repository.NewCoffeeBeanRepository(db),
		repository.NewUserRepository(db),
		logger,
	)
	return svc, db
}

func seedBlindUsers(t *testing.T, svc *BlindTastingService) (*model.CoffeeBean, []model.User) {
	t.Helper()
	// reach into the repos through service deps via fresh seed: use bean/user create directly
	bean := &model.CoffeeBean{Name: "测试豆 耶加雪菲", Origin: "埃塞俄比亚", ProcessMethod: constants.ProcessWashed, FlavorTags: "[]"}
	if err := svc.beanRepo.Create(bean); err != nil {
		t.Fatalf("create bean: %v", err)
	}
	users := make([]model.User, 0, 4)
	for i := 0; i < 4; i++ {
		u := &model.User{Username: "taster" + string(rune('A'+i)), Email: "t" + string(rune('A'+i)) + "@x.local", PasswordHash: "x", Role: "user"}
		if err := svc.userRepo.Create(u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		users = append(users, *u)
	}
	return bean, users
}

func happyScore(v float64) dto.BlindScoreSubmitRequest {
	return dto.BlindScoreSubmitRequest{AromaScore: v, AcidityScore: v, BodyScore: v, OverallScore: v}
}

func TestBlindClosedLoopHappyPath(t *testing.T) {
	svc, _ := newBlindTestService(t)
	bean, users := seedBlindUsers(t, svc)
	host := users[0]
	p1, p2, p3 := users[1], users[2], users[3]

	session, err := svc.Create(host.ID, bean.ID, []uint{p1.ID, p2.ID, p3.ID})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if session.Status != constants.BlindStatusOngoing {
		t.Fatalf("unexpected status %s", session.Status)
	}

	// Before any submission: host sees zero scores, participant sees no numbers of others.
	view, err := svc.Get(host.ID, session.ID)
	if err != nil {
		t.Fatalf("get host: %v", err)
	}
	if view.SubmittedCount != 0 || len(view.Scores) != 3 {
		t.Fatalf("unexpected initial view: %+v", view)
	}

	scores := []dto.BlindScoreSubmitRequest{
		{AromaScore: 8, AcidityScore: 7, BodyScore: 6, OverallScore: 7.5},
		{AromaScore: 9, AcidityScore: 8, BodyScore: 7, OverallScore: 8},
		{AromaScore: 6, AcidityScore: 7.5, BodyScore: 6.5, OverallScore: 7},
	}
	participants := []uint{p1.ID, p2.ID, p3.ID}
	for i, pid := range participants {
		if _, err := svc.SubmitScore(pid, session.ID, scores[i]); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}

	// Anonymity: before reveal a participant cannot read others' numeric scores.
	p1View, err := svc.Get(p1.ID, session.ID)
	if err != nil {
		t.Fatalf("get p1: %v", err)
	}
	if p1View.Averages != nil {
		t.Fatal("averages must stay hidden before reveal")
	}
	for _, row := range p1View.Scores {
		if row.UserID == p1.ID {
			if row.AromaScore != 8 {
				t.Fatalf("own score should be visible: %+v", row)
			}
			continue
		}
		if !row.Submitted || row.AromaScore != 0 || row.OverallScore != 0 {
			t.Fatalf("other score leaked before reveal: %+v", row)
		}
	}

	revealed, err := svc.Reveal(host.ID, session.ID)
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	if revealed.Status != constants.BlindStatusRevealed || revealed.Averages == nil {
		t.Fatalf("bad revealed view: %+v", revealed)
	}
	// aroma mean: (8+9+6)/3 = 7.67; body mean: (6+7+6.5)/3 = 6.5
	if revealed.Averages.Aroma != 7.67 || revealed.Averages.Body != 6.5 {
		t.Fatalf("unexpected averages: %+v", revealed.Averages)
	}
	// Outlier rule: p3 aroma 6 deviates 1.67 > 1.5; exactly-1.5 deviations are not outliers.
	byUser := map[uint]dto.BlindScoreView{}
	for _, s := range revealed.Scores {
		byUser[s.UserID] = s
	}
	if !byUser[p3.ID].AromaOutlier {
		t.Fatal("p3 aroma 6 should be outlier vs mean 7.67")
	}
	if byUser[p3.ID].BodyOutlier {
		t.Fatal("p3 body 6.5 equals mean, not outlier")
	}
	// p2 aroma 9 deviates 1.33, not outlier
	if byUser[p2.ID].AromaOutlier {
		t.Fatal("p2 aroma 9 must not be outlier vs mean 7.67")
	}
}

func TestBlindCreateValidationFailures(t *testing.T) {
	svc, _ := newBlindTestService(t)
	bean, users := seedBlindUsers(t, svc)
	host := users[0]
	p1, p2, p3 := users[1], users[2], users[3]

	cases := []struct {
		name string
		ids  []uint
	}{
		{"too few", []uint{p1.ID, p2.ID}},
		{"too many", []uint{p1.ID, p2.ID, p3.ID, host.ID}},
		{"duplicate", []uint{p1.ID, p1.ID, p2.ID}},
		{"host included", []uint{host.ID, p1.ID, p2.ID}},
		{"unknown user", []uint{p1.ID, p2.ID, 9999}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(host.ID, bean.ID, tc.ids); err == nil {
				t.Fatalf("expected failure for %s", tc.name)
			}
		})
	}
	if _, err := svc.Create(host.ID, 9999, []uint{p1.ID, p2.ID, p3.ID}); err == nil {
		t.Fatal("expected failure for unknown bean")
	}
}

func TestBlindSubmissionFailuresLeaveDataIntact(t *testing.T) {
	svc, _ := newBlindTestService(t)
	bean, users := seedBlindUsers(t, svc)
	host := users[0]
	p1, p2, p3 := users[1], users[2], users[3]
	session, err := svc.Create(host.ID, bean.ID, []uint{p1.ID, p2.ID, p3.ID})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Outsider cannot submit.
	if _, err := svc.SubmitScore(host.ID, session.ID, happyScore(8)); err == nil {
		t.Fatal("host (not a participant) must not submit")
	}
	// Out of range rejected.
	if _, err := svc.SubmitScore(p1.ID, session.ID, happyScore(11)); err == nil {
		t.Fatal("score > 10 must be rejected")
	}
	// Original data unchanged: still zero submissions.
	view, _ := svc.Get(host.ID, session.ID)
	if view.SubmittedCount != 0 {
		t.Fatalf("failed submissions must not mutate data, count=%d", view.SubmittedCount)
	}

	// One valid submission.
	if _, err := svc.SubmitScore(p1.ID, session.ID, happyScore(8)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// Duplicate submission rejected.
	_, err = svc.SubmitScore(p1.ID, session.ID, happyScore(9))
	var appErr *util.AppError
	if !errors.As(err, &appErr) || appErr.Code != constants.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
	// Original score unchanged.
	after, _ := svc.Get(p1.ID, session.ID)
	for _, row := range after.Scores {
		if row.UserID == p1.ID && row.AromaScore != 8 {
			t.Fatalf("duplicate submission mutated original data: %+v", row)
		}
	}

	// Reveal before all submissions fails.
	if _, err := svc.Reveal(host.ID, session.ID); err == nil {
		t.Fatal("reveal with missing submissions must fail")
	}
	// Non-host reveal fails.
	if _, err := svc.SubmitScore(p2.ID, session.ID, happyScore(8)); err != nil {
		t.Fatalf("submit p2: %v", err)
	}
	if _, err := svc.SubmitScore(p3.ID, session.ID, happyScore(8)); err != nil {
		t.Fatalf("submit p3: %v", err)
	}
	if _, err := svc.Reveal(p1.ID, session.ID); err == nil {
		t.Fatal("non-host reveal must fail")
	}
	// Successful reveal.
	if _, err := svc.Reveal(host.ID, session.ID); err != nil {
		t.Fatalf("reveal: %v", err)
	}
	// Repeated reveal fails.
	if _, err := svc.Reveal(host.ID, session.ID); err == nil {
		t.Fatal("repeat reveal must fail")
	}
	// Submission after reveal fails and leaves data intact.
	_, err = svc.SubmitScore(p1.ID, session.ID, happyScore(5))
	if err == nil {
		t.Fatal("submission after reveal must fail")
	}
	view2, _ := svc.Get(host.ID, session.ID)
	for _, row := range view2.Scores {
		if row.UserID == p1.ID && row.AromaScore != 8 {
			t.Fatalf("post-reveal submit mutated data: %+v", row)
		}
	}
}

func TestBlindAccessControl(t *testing.T) {
	svc, _ := newBlindTestService(t)
	bean, users := seedBlindUsers(t, svc)
	host, p1, p2, p3 := users[0], users[1], users[2], users[3]
	outsider := &model.User{Username: "outsider", Email: "o@x.local", PasswordHash: "x", Role: "user"}
	if err := svc.userRepo.Create(outsider); err != nil {
		t.Fatalf("create outsider: %v", err)
	}
	session, err := svc.Create(host.ID, bean.ID, []uint{p1.ID, p2.ID, p3.ID})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.Get(outsider.ID, session.ID); err == nil {
		t.Fatal("outsider must not read the session")
	}
	if _, err := svc.Get(0, session.ID); err == nil {
		t.Fatal("anonymous request must not read the session")
	}
}

func TestBlindConcurrentSubmissions(t *testing.T) {
	svc, _ := newBlindTestService(t)
	bean, users := seedBlindUsers(t, svc)
	host, p1, p2, p3 := users[0], users[1], users[2], users[3]
	session, err := svc.Create(host.ID, bean.ID, []uint{p1.ID, p2.ID, p3.ID})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Fire many parallel submissions for the same participant: exactly one may win.
	const n = 12
	var wg sync.WaitGroup
	var okCount, conflictCount int32
	var mu sync.Mutex
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.SubmitScore(p1.ID, session.ID, happyScore(7))
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				okCount++
			} else {
				var appErr *util.AppError
				if errors.As(err, &appErr) && appErr.Code == constants.CodeConflict {
					conflictCount++
				}
			}
		}()
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("expected exactly 1 successful submission, got %d (conflicts=%d)", okCount, conflictCount)
	}
	if conflictCount != n-1 {
		t.Fatalf("expected %d conflicts, got %d", n-1, conflictCount)
	}
	view, _ := svc.Get(host.ID, session.ID)
	if view.SubmittedCount != 1 {
		t.Fatalf("concurrent submissions mutated data unexpectedly: count=%d", view.SubmittedCount)
	}

	// Complete the round, then run parallel reveals: exactly one may win.
	if _, err := svc.SubmitScore(p2.ID, session.ID, happyScore(8)); err != nil {
		t.Fatalf("submit p2: %v", err)
	}
	if _, err := svc.SubmitScore(p3.ID, session.ID, happyScore(9)); err != nil {
		t.Fatalf("submit p3: %v", err)
	}
	var revealOK, revealFail int32
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.Reveal(host.ID, session.ID)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				revealOK++
			} else {
				revealFail++
			}
		}()
	}
	wg.Wait()
	if revealOK != 1 || revealFail != n-1 {
		t.Fatalf("expected 1 successful reveal and %d failures, got ok=%d fail=%d", n-1, revealOK, revealFail)
	}
}
