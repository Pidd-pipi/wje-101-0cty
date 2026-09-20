package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/config"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type blindAPIEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"message"`
	Data json.RawMessage `json:"data"`
}

func setupBlindAPITest(t *testing.T) (*gin.Engine, *gorm.DB, map[string]string) {
	t.Helper()
	dsn := fmt.Sprintf("file:api_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{}, &model.CoffeeBean{},
		&model.BlindTastingSession{}, &model.BlindTastingParticipant{}, &model.BlindScore{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// host + 3 tasters + 1 outsider
	mk := func(id uint, name string) *model.User {
		return &model.User{ID: id, Username: name, Email: name + "@x.local", PasswordHash: "x", Role: "user"}
	}
	users := []*model.User{mk(1, "host"), mk(2, "a"), mk(3, "b"), mk(4, "c"), mk(5, "outsider")}
	for _, u := range users {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	bean := &model.CoffeeBean{ID: 1, Name: "瑰夏", Origin: "巴拿马", ProcessMethod: "washed", FlavorTags: "[]"}
	if err := db.Create(bean).Error; err != nil {
		t.Fatalf("seed bean: %v", err)
	}

	cfg := &config.Config{
		JWTSecret:    "test-secret",
		JWTExpire:    time.Hour,
		RateLimitReq: 100000,
		RateLimitWin: time.Minute,
	}
	r := Setup(cfg, db, util.NewLogger())

	tokens := map[string]string{}
	for _, role := range []struct {
		name string
		id   uint
		user string
	}{
		{"host", 1, "host"}, {"a", 2, "a"}, {"b", 3, "b"}, {"c", 4, "c"}, {"outsider", 5, "outsider"},
	} {
		tok, err := util.GenerateToken(role.id, role.user, "user", cfg.JWTSecret, cfg.JWTExpire)
		if err != nil {
			t.Fatalf("token: %v", err)
		}
		tokens[role.name] = tok
	}
	return r, db, tokens
}

func doBlindJSON(t *testing.T, r *gin.Engine, method, path, token string, body any) (int, blindAPIEnvelope) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env blindAPIEnvelope
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("decode response %q: %v", w.Body.String(), err)
		}
	}
	return w.Code, env
}

func blindSessionID(t *testing.T, data json.RawMessage) uint {
	t.Helper()
	var m struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode id: %v", err)
	}
	return m.ID
}

func TestBlindAPIEndToEnd(t *testing.T) {
	r, _, tokens := setupBlindAPITest(t)

	// Unauthenticated list rejected.
	if code, _ := doBlindJSON(t, r, "GET", "/api/v1/blind-tastings", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}

	// Wrong participant count rejected (422).
	code, env := doBlindJSON(t, r, "POST", "/api/v1/blind-tastings", tokens["host"],
		map[string]any{"coffee_bean_id": 1, "participant_ids": []uint{2, 3}})
	if code != http.StatusUnprocessableEntity || env.Code == 0 {
		t.Fatalf("expected 422 for too few participants, got %d env=%+v", code, env)
	}

	// Create session.
	code, env = doBlindJSON(t, r, "POST", "/api/v1/blind-tastings", tokens["host"],
		map[string]any{"coffee_bean_id": 1, "participant_ids": []uint{2, 3, 4}})
	if code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", code, env.Msg)
	}
	sid := blindSessionID(t, env.Data)

	// Outsider cannot see session (403).
	if code, _ := doBlindJSON(t, r, "GET", "/api/v1/blind-tastings/"+fmt.Sprint(sid), tokens["outsider"], nil); code != http.StatusForbidden {
		t.Fatalf("expected 403 for outsider, got %d", code)
	}

	// Submit once per participant.
	submit := func(tok string, v float64) (int, blindAPIEnvelope) {
		return doBlindJSON(t, r, "POST", fmt.Sprintf("/api/v1/blind-tastings/%d/scores", sid), tok,
			map[string]float64{"aroma_score": v, "acidity_score": v, "body_score": v, "overall_score": v})
	}
	if code, env := submit(tokens["a"], 8); code != http.StatusCreated {
		t.Fatalf("a submit: %d %s", code, env.Msg)
	}
	// Duplicate submission -> 409.
	if code, _ := submit(tokens["a"], 9); code != http.StatusConflict {
		t.Fatalf("expected 409 on duplicate, got %d", code)
	}
	// Invalid score -> 422.
	if code, _ := doBlindJSON(t, r, "POST", fmt.Sprintf("/api/v1/blind-tastings/%d/scores", sid), tokens["b"],
		map[string]float64{"aroma_score": 12, "acidity_score": 8, "body_score": 8, "overall_score": 8}); code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for out-of-range score, got %d", code)
	}
	if code, _ := submit(tokens["b"], 8.5); code != http.StatusCreated {
		t.Fatalf("b submit: %d", code)
	}

	// Host attempts reveal too early -> 409.
	if code, _ := doBlindJSON(t, r, "POST", fmt.Sprintf("/api/v1/blind-tastings/%d/reveal", sid), tokens["host"], nil); code != http.StatusConflict {
		t.Fatalf("expected 409 early reveal, got %d", code)
	}
	// Non-host reveal -> 403.
	if code, _ := doBlindJSON(t, r, "POST", fmt.Sprintf("/api/v1/blind-tastings/%d/reveal", sid), tokens["a"], nil); code != http.StatusForbidden {
		t.Fatalf("expected 403 non-host reveal, got %d", code)
	}

	// Anonymity check: host's GET during ongoing must expose no numeric scores.
	code, env = doBlindJSON(t, r, "GET", fmt.Sprintf("/api/v1/blind-tastings/%d", sid), tokens["host"], nil)
	if code != http.StatusOK {
		t.Fatalf("get: %d", code)
	}
	var view struct {
		Status string `json:"status"`
		Scores []struct {
			Submitted bool    `json:"submitted"`
			Aroma     float64 `json:"aroma_score"`
			Overall   float64 `json:"overall_score"`
		} `json:"scores"`
		Averages any `json:"averages"`
	}
	if err := json.Unmarshal(env.Data, &view); err != nil {
		t.Fatalf("decode view: %v", err)
	}
	if view.Averages != nil {
		t.Fatal("averages must be absent before reveal")
	}
	for _, row := range view.Scores {
		if row.Submitted && (row.Aroma != 0 || row.Overall != 0) {
			t.Fatalf("scores leaked to host before reveal: %+v", row)
		}
	}

	// Participant a can still read their own numbers.
	code, env = doBlindJSON(t, r, "GET", fmt.Sprintf("/api/v1/blind-tastings/%d", sid), tokens["a"], nil)
	if code != http.StatusOK {
		t.Fatalf("get as participant: %d", code)
	}
	var selfView struct {
		Scores []struct {
			UserID uint    `json:"user_id"`
			Aroma  float64 `json:"aroma_score"`
		} `json:"scores"`
	}
	_ = json.Unmarshal(env.Data, &selfView)
	foundOwn := false
	for _, row := range selfView.Scores {
		if row.UserID == 2 && row.Aroma == 8 {
			foundOwn = true
		}
	}
	if !foundOwn {
		t.Fatal("participant cannot see own score during ongoing phase")
	}

	// Last submission + reveal (5.8 deviates from aroma mean 7.43 by 1.63 > 1.5).
	if code, _ := submit(tokens["c"], 5.8); code != http.StatusCreated {
		t.Fatalf("c submit: %d", code)
	}
	code, env = doBlindJSON(t, r, "POST", fmt.Sprintf("/api/v1/blind-tastings/%d/reveal", sid), tokens["host"], nil)
	if code != http.StatusOK {
		t.Fatalf("reveal: %d %s", code, env.Msg)
	}
	// Repeat reveal -> 409.
	if code, _ := doBlindJSON(t, r, "POST", fmt.Sprintf("/api/v1/blind-tastings/%d/reveal", sid), tokens["host"], nil); code != http.StatusConflict {
		t.Fatalf("repeat reveal expected 409, got %d", code)
	}
	// Submission after reveal -> 409.
	if code, _ := submit(tokens["a"], 5); code != http.StatusConflict {
		t.Fatalf("post-reveal submit expected 409, got %d", code)
	}

	// Final GET: averages and outliers present.
	code, env = doBlindJSON(t, r, "GET", fmt.Sprintf("/api/v1/blind-tastings/%d", sid), tokens["host"], nil)
	if code != http.StatusOK {
		t.Fatalf("final get: %d", code)
	}
	var final struct {
		Status   string `json:"status"`
		Averages struct {
			Aroma float64 `json:"aroma"`
		} `json:"averages"`
		Scores []struct {
			UserID       uint `json:"user_id"`
			AromaOutlier bool `json:"aroma_outlier"`
		} `json:"scores"`
	}
	if err := json.Unmarshal(env.Data, &final); err != nil {
		t.Fatalf("decode final: %v", err)
	}
	// aroma mean of 8, 8.5, 5.8 = 7.43
	if final.Status != "revealed" || final.Averages.Aroma != 7.43 {
		t.Fatalf("unexpected final payload: %+v", final)
	}
	outliers := map[uint]bool{}
	for _, row := range final.Scores {
		outliers[row.UserID] = row.AromaOutlier
	}
	if !outliers[4] {
		t.Fatal("participant c aroma 5.8 must be flagged outlier")
	}
	if outliers[2] || outliers[3] {
		t.Fatal("participants a/b aroma are within 1.5 of the mean")
	}
}

func TestBlindAPICreateDuplicateParticipants(t *testing.T) {
	r, _, tokens := setupBlindAPITest(t)
	code, _ := doBlindJSON(t, r, "POST", "/api/v1/blind-tastings", tokens["host"],
		map[string]any{"coffee_bean_id": 1, "participant_ids": []uint{2, 2, 3}})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate participants expected 422, got %d", code)
	}
}
