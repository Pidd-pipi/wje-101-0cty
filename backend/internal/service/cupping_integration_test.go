package service

// These integration tests exercise the cupping repositories + service against
// a real SQL database (pure-Go SQLite via github.com/glebarez/sqlite) so they
// run without a PostgreSQL server or a C toolchain. They verify transactions,
// the unique (cupping_id,user_id) index (duplicate/concurrent submission) and
// the full reveal closed loop. Production runs on PostgreSQL, where the same
// unique index plus SELECT ... FOR UPDATE row locks provide identical and
// stronger guarantees.

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/dto"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/repository"
)

func newCuppingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "cupping.db") + "?_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.CoffeeBean{},
		&model.BlindCupping{}, &model.CuppingParticipant{}, &model.BlindScore{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newCuppingFixture(t *testing.T) (*gorm.DB, *CuppingService, []model.User, model.CoffeeBean) {
	db := newCuppingTestDB(t)
	users := []model.User{
		{Username: "org", Email: "org@x.local", PasswordHash: "x", Role: "user"},
		{Username: "p1", Email: "p1@x.local", PasswordHash: "x", Role: "user"},
		{Username: "p2", Email: "p2@x.local", PasswordHash: "x", Role: "user"},
		{Username: "p3", Email: "p3@x.local", PasswordHash: "x", Role: "user"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	bean := model.CoffeeBean{Name: "测试豆", Origin: "云南", ProcessMethod: "washed", FlavorTags: "[]"}
	if err := db.Create(&bean).Error; err != nil {
		t.Fatalf("create bean: %v", err)
	}
	svc := NewCuppingService(
		repository.NewBlindCuppingRepository(db),
		repository.NewCuppingParticipantRepository(db),
		repository.NewBlindScoreRepository(db),
		repository.NewCoffeeBeanRepository(db),
		repository.NewUserRepository(db),
		newTestLogger(),
	)
	return db, svc, users, bean
}

func scoreReq(a, c, b, o float64) dto.CuppingScoreRequest {
	return dto.CuppingScoreRequest{AromaScore: a, AcidityScore: c, BodyScore: b, OverallScore: o}
}

func TestCuppingHappyFlow(t *testing.T) {
	_, svc, users, bean := newCuppingFixture(t)
	org, p1, p2, p3 := users[0], users[1], users[2], users[3]

	cup, err := svc.Create(org.ID, bean.ID, []uint{p1.ID, p2.ID, p3.ID})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if cup.Status != constants.CuppingCollecting || cup.SubmittedCount != 0 {
		t.Fatalf("unexpected initial view: %+v", cup)
	}
	if len(cup.Scores) != 0 {
		t.Fatalf("scores must be hidden before reveal")
	}

	if _, err := svc.Submit(cup.ID, p1.ID, scoreReq(8, 7, 7, 8)); err != nil {
		t.Fatalf("p1 submit: %v", err)
	}
	// p2's view must still hide scores (blind).
	view, err := svc.Get(cup.ID, p2.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(view.Scores) != 0 {
		t.Fatalf("scores leaked before reveal: %+v", view.Scores)
	}
	if _, err := svc.Submit(cup.ID, p2.ID, scoreReq(9, 7, 7, 8)); err != nil {
		t.Fatalf("p2 submit: %v", err)
	}

	// reveal before all submitted must fail and leave data intact.
	if _, err := svc.Reveal(cup.ID, org.ID); err == nil {
		t.Fatalf("reveal with missing submission should fail")
	}
	// p3 diverges only on aroma (4 vs 8-ish), other dimensions stay aligned.
	if _, err := svc.Submit(cup.ID, p3.ID, scoreReq(4, 7, 7, 8)); err != nil {
		t.Fatalf("p3 submit: %v", err)
	}

	// non-organizer cannot reveal.
	if _, err := svc.Reveal(cup.ID, p1.ID); err == nil {
		t.Fatalf("non-organizer reveal should fail")
	}

	revealed, err := svc.Reveal(cup.ID, org.ID)
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	if revealed.Status != constants.CuppingRevealed {
		t.Fatalf("status = %s", revealed.Status)
	}
	// aroma (8+9+4)/3=7.0; acidity/body/overall are unanimous: 7.0/7.0/8.0
	if revealed.AvgAroma != 7.0 || revealed.AvgAcidity != 7.0 || revealed.AvgBody != 7.0 || revealed.AvgOverall != 8.0 {
		t.Fatalf("unexpected averages: aroma=%v acidity=%v body=%v overall=%v",
			revealed.AvgAroma, revealed.AvgAcidity, revealed.AvgBody, revealed.AvgOverall)
	}
	if len(revealed.Scores) != 3 {
		t.Fatalf("expected 3 revealed scores, got %d", len(revealed.Scores))
	}
	// p3 aroma 4 vs avg 7.0 -> deviation 3.0 outlier; other three dimensions at average.
	var p3view *dto.CuppingScoreView
	for i := range revealed.Scores {
		if revealed.Scores[i].UserID == p3.ID {
			p3view = &revealed.Scores[i]
		}
	}
	if p3view == nil {
		t.Fatal("p3 score missing after reveal")
	}
	if !p3view.OutlierAroma {
		t.Fatalf("p3 aroma should be outlier: %+v", p3view)
	}
	if p3view.OutlierAcidity || p3view.OutlierBody || p3view.OutlierOverall {
		t.Fatalf("p3 aligned dimensions must not be outliers: %+v", p3view)
	}
	// p1 aroma 8 vs 7.0 = 1.0, within threshold; p2 aroma 9 vs 7.0 = 2.0 outlier.
	var p1view, p2view *dto.CuppingScoreView
	for i := range revealed.Scores {
		switch revealed.Scores[i].UserID {
		case p1.ID:
			p1view = &revealed.Scores[i]
		case p2.ID:
			p2view = &revealed.Scores[i]
		}
	}
	if p1view == nil || p1view.OutlierAroma {
		t.Fatalf("p1 aroma (deviation 1.0) must not be outlier: %+v", p1view)
	}
	if p2view == nil || !p2view.OutlierAroma {
		t.Fatalf("p2 aroma (deviation 2.0) should be outlier: %+v", p2view)
	}
}

func TestCuppingCreateValidation(t *testing.T) {
	_, svc, users, bean := newCuppingFixture(t)
	org, p1, p2 := users[0], users[1], users[2]

	if _, err := svc.Create(org.ID, bean.ID, []uint{p1.ID, p2.ID}); err == nil {
		t.Fatal("fewer than 3 participants should fail")
	}
	if _, err := svc.Create(org.ID, bean.ID, []uint{p1.ID, p1.ID, p2.ID}); err == nil {
		t.Fatal("duplicate participants should fail")
	}
	if _, err := svc.Create(org.ID, 99999, []uint{p1.ID, p2.ID, users[3].ID}); err == nil {
		t.Fatal("missing bean should fail")
	}
	if _, err := svc.Create(org.ID, bean.ID, []uint{p1.ID, p2.ID, 77777}); err == nil {
		t.Fatal("nonexistent participant should fail")
	}
}

func TestCuppingSubmitGuards(t *testing.T) {
	_, svc, users, bean := newCuppingFixture(t)
	org, p1, p2, p3 := users[0], users[1], users[2], users[3]
	cup, err := svc.Create(org.ID, bean.ID, []uint{p1.ID, p2.ID, p3.ID})
	if err != nil {
		t.Fatal(err)
	}

	// non-participant cannot submit.
	if _, err := svc.Submit(cup.ID, org.ID, scoreReq(8, 8, 8, 8)); err == nil {
		t.Fatal("non-participant submit should fail")
	}
	// out-of-range score rejected.
	if _, err := svc.Submit(cup.ID, p1.ID, scoreReq(11, 8, 8, 8)); err == nil {
		t.Fatal("score above 10 should fail")
	}
	if _, err := svc.Submit(cup.ID, p1.ID, scoreReq(8, 8, 8, 8)); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	// duplicate submit fails and original data is unchanged.
	if _, err := svc.Submit(cup.ID, p1.ID, scoreReq(1, 1, 1, 1)); err == nil {
		t.Fatal("duplicate submit should fail")
	}
	got, err := svc.Get(cup.ID, p1.ID)
	if err != nil {
		t.Fatal(err)
	}
	// still collecting and the rejected value did not replace the 8.0 submission.
	if got.Status != constants.CuppingCollecting {
		t.Fatalf("status changed after failed submit: %s", got.Status)
	}
}

func TestCuppingRevealIdempotencyFails(t *testing.T) {
	_, svc, users, bean := newCuppingFixture(t)
	org, p1, p2, p3 := users[0], users[1], users[2], users[3]
	cup, _ := svc.Create(org.ID, bean.ID, []uint{p1.ID, p2.ID, p3.ID})
	_, _ = svc.Submit(cup.ID, p1.ID, scoreReq(8, 8, 8, 8))
	_, _ = svc.Submit(cup.ID, p2.ID, scoreReq(8, 8, 8, 8))
	_, _ = svc.Submit(cup.ID, p3.ID, scoreReq(8, 8, 8, 8))
	if _, err := svc.Reveal(cup.ID, org.ID); err != nil {
		t.Fatal(err)
	}
	// second reveal must fail.
	if _, err := svc.Reveal(cup.ID, org.ID); err == nil {
		t.Fatal("duplicate reveal should fail")
	}
	// no submissions accepted after reveal.
	if _, err := svc.Submit(cup.ID, p1.ID, scoreReq(8, 8, 8, 8)); err == nil {
		t.Fatal("submit after reveal should fail")
	}
}

// TestCuppingConcurrentSubmit races two identical submissions: exactly one wins.
// SQLite's single-writer model plus the unique (cupping_id,user_id) index
// guarantees the duplicate is rejected; on PostgreSQL the row lock serializes
// them and the unique index is the backstop.
func TestCuppingConcurrentSubmit(t *testing.T) {
	_, svc, users, bean := newCuppingFixture(t)
	org, p1, p2, p3 := users[0], users[1], users[2], users[3]
	cup, _ := svc.Create(org.ID, bean.ID, []uint{p1.ID, p2.ID, p3.ID})

	var wg sync.WaitGroup
	var mu sync.Mutex
	var success, conflict int
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Submit(cup.ID, p1.ID, scoreReq(8, 8, 8, 8))
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
			} else {
				conflict++
			}
		}()
	}
	wg.Wait()
	if success != 1 || conflict != 1 {
		t.Fatalf("concurrent duplicate submit: success=%d conflict=%d, want 1/1", success, conflict)
	}
}
