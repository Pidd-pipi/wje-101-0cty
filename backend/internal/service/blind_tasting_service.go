package service

import (
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/dto"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/repository"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/util"
)

// BlindTastingService runs the blind cupping closed loop:
// create -> anonymous one-shot submissions -> host reveal -> averages/outliers.
type BlindTastingService struct {
	repo     *repository.BlindTastingRepository
	beanRepo *repository.CoffeeBeanRepository
	userRepo *repository.UserRepository
	logger   *slog.Logger
}

// NewBlindTastingService creates the service.
func NewBlindTastingService(repo *repository.BlindTastingRepository, beanRepo *repository.CoffeeBeanRepository, userRepo *repository.UserRepository, logger *slog.Logger) *BlindTastingService {
	return &BlindTastingService{repo: repo, beanRepo: beanRepo, userRepo: userRepo, logger: logger}
}

// Create starts a round for hostID on a bean with exactly three tasters.
func (s *BlindTastingService) Create(hostID uint, beanID uint, participantIDs []uint) (*model.BlindTastingSession, error) {
	if beanID == 0 {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			"BlindTastingSession[coffee_bean_id=0] create failed: bean is required")
	}
	if err := validateBlindParticipants(hostID, participantIDs); err != nil {
		return nil, err
	}
	bean, err := s.beanRepo.FindByID(beanID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("CoffeeBean[id=%d] not found", beanID))
		}
		return nil, fmt.Errorf("blind create bean: %w", err)
	}
	users, err := s.userRepo.FindByIDs(participantIDs)
	if err != nil {
		return nil, fmt.Errorf("blind create participants: %w", err)
	}
	if len(users) != constants.BlindParticipantCount {
		s.logger.Warn(fmt.Sprintf(constants.LogBlindCreateFailed, hostID, beanID),
			"reason", "participant not found", "want", constants.BlindParticipantCount, "got", len(users))
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("BlindTastingSession[host_id=%d role=user] create failed: %d participants not found",
				hostID, constants.BlindParticipantCount-len(users)))
	}
	session := &model.BlindTastingSession{
		HostID:         hostID,
		CoffeeBeanID:   bean.ID,
		CoffeeBeanName: bean.Name,
		Status:         constants.BlindStatusOngoing,
	}
	if err := s.repo.Create(session, participantIDs); err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogBlindCreateFailed, hostID, beanID), "error", err)
		return nil, fmt.Errorf("blind create: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogBlindCreateSuccess, session.ID, hostID, beanID))
	return session, nil
}

// validateBlindParticipants enforces: exactly three, unique, excluding the host.
func validateBlindParticipants(hostID uint, participantIDs []uint) error {
	if len(participantIDs) != constants.BlindParticipantCount {
		return util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("BlindTastingSession[host_id=%d] create failed: requires exactly %d participants, got %d",
				hostID, constants.BlindParticipantCount, len(participantIDs)))
	}
	seen := make(map[uint]struct{}, constants.BlindParticipantCount)
	for _, pid := range participantIDs {
		if pid == 0 {
			return util.NewAppError(422, constants.CodeValidationError,
				"BlindTastingSession participant_ids[] invalid: user id must be positive")
		}
		if pid == hostID {
			return util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("BlindTastingSession[host_id=%d] create failed: host cannot be a participant", hostID))
		}
		if _, dup := seen[pid]; dup {
			return util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("BlindTastingSession[user_id=%d] create failed: duplicate participant", pid))
		}
		seen[pid] = struct{}{}
	}
	return nil
}

// SubmitScore records a participant's only submission for a session.
func (s *BlindTastingService) SubmitScore(userID, sessionID uint, req dto.BlindScoreSubmitRequest) (*model.BlindScore, error) {
	if !constants.IsValidBlindScore(req.AromaScore) || !constants.IsValidBlindScore(req.AcidityScore) ||
		!constants.IsValidBlindScore(req.BodyScore) || !constants.IsValidBlindScore(req.OverallScore) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("BlindScore[session_id=%d user_id=%d] submit failed: every score must be within %.0f-%.0f",
				sessionID, userID, constants.BlindMinScore, constants.BlindMaxScore))
	}
	if _, err := s.repo.FindSession(sessionID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("BlindTastingSession[id=%d] not found", sessionID))
		}
		return nil, fmt.Errorf("blind score session: %w", err)
	}
	if !s.isParticipant(sessionID, userID) {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("BlindScore[session_id=%d user_id=%d] submit failed: not a participant", sessionID, userID))
	}
	if existing, err := s.repo.FindScore(sessionID, userID); err == nil && existing != nil {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("BlindScore[session_id=%d user_id=%d] submit failed: already submitted", sessionID, userID))
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("blind score check: %w", err)
	}
	score := &model.BlindScore{
		SessionID:    sessionID,
		UserID:       userID,
		AromaScore:   req.AromaScore,
		AcidityScore: req.AcidityScore,
		BodyScore:    req.BodyScore,
		OverallScore: req.OverallScore,
	}
	if err := s.repo.SubmitScore(score); err != nil {
		return nil, s.mapSubmitError(sessionID, userID, err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogBlindScoreSuccess, sessionID, userID))
	return score, nil
}

func (s *BlindTastingService) mapSubmitError(sessionID, userID uint, err error) error {
	switch {
	case errors.Is(err, repository.ErrDuplicate):
		// Concurrent double submission raced past the pre-check.
		return util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("BlindScore[session_id=%d user_id=%d] submit failed: already submitted", sessionID, userID))
	case errors.Is(err, repository.ErrAlreadyRevealed):
		return util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("BlindScore[session_id=%d] submit failed: session already revealed", sessionID))
	case errors.Is(err, repository.ErrNotFound):
		return util.NewAppError(404, constants.CodeNotFound,
			fmt.Sprintf("BlindTastingSession[id=%d] not found", sessionID))
	default:
		s.logger.Error(fmt.Sprintf(constants.LogBlindScoreFailed, sessionID, userID), "error", err)
		return fmt.Errorf("blind score submit: %w", err)
	}
}

// Reveal closes the round. Only the host may reveal, only once, and only after
// every participant has submitted.
func (s *BlindTastingService) Reveal(userID, sessionID uint) (*dto.BlindSessionView, error) {
	session, err := s.repo.FindSession(sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("BlindTastingSession[id=%d] not found", sessionID))
		}
		return nil, fmt.Errorf("blind reveal session: %w", err)
	}
	if session.HostID != userID {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("BlindTastingSession[id=%d] reveal failed: user_id=%d not host role=user", sessionID, userID))
	}
	if session.Status == constants.BlindStatusRevealed {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("BlindTastingSession[id=%d] reveal failed: already revealed", sessionID))
	}
	if err := s.repo.Reveal(sessionID); err != nil {
		if errors.Is(err, repository.ErrAlreadyRevealed) {
			return nil, util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("BlindTastingSession[id=%d] reveal failed: already revealed", sessionID))
		}
		if errors.Is(err, repository.ErrNotAllSubmitted) {
			return nil, util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("BlindTastingSession[id=%d] reveal failed: not all participants submitted", sessionID))
		}
		s.logger.Error(fmt.Sprintf(constants.LogBlindRevealFailed, sessionID, userID), "error", err)
		return nil, fmt.Errorf("blind reveal: %w", err)
	}
	s.logger.Info(fmt.Sprintf(constants.LogBlindRevealSuccess, sessionID, userID))
	return s.Get(userID, sessionID)
}

// Get returns the session view for a viewer. Hosts and participants are
// admitted. Before reveal, other participants' numeric scores are masked.
func (s *BlindTastingService) Get(viewerID, sessionID uint) (*dto.BlindSessionView, error) {
	session, err := s.repo.FindSession(sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("BlindTastingSession[id=%d] not found", sessionID))
		}
		return nil, fmt.Errorf("blind get session: %w", err)
	}
	participants, err := s.repo.ListParticipants(sessionID)
	if err != nil {
		return nil, fmt.Errorf("blind get participants: %w", err)
	}
	scores, err := s.repo.ListScores(sessionID)
	if err != nil {
		return nil, fmt.Errorf("blind get scores: %w", err)
	}

	isHost := session.HostID == viewerID
	participantSet := make(map[uint]bool, len(participants))
	ids := make([]uint, 0, len(participants))
	for _, p := range participants {
		participantSet[p.UserID] = true
		ids = append(ids, p.UserID)
	}
	if !isHost && !participantSet[viewerID] {
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("BlindTastingSession[id=%d] access failed: user_id=%d is neither host nor participant",
				sessionID, viewerID))
	}

	userMap, err := s.loadUsers(append(ids, session.HostID))
	if err != nil {
		return nil, err
	}

	scoreByUser := make(map[uint]model.BlindScore, len(scores))
	for _, sc := range scores {
		scoreByUser[sc.UserID] = sc
	}

	revealed := session.Status == constants.BlindStatusRevealed
	averages := computeBlindAverages(scores)

	views := make([]dto.BlindScoreView, 0, len(ids))
	for _, uid := range ids {
		view := dto.BlindScoreView{UserID: uid}
		if u, ok := userMap[uid]; ok {
			view.Username = u.Username
			view.Avatar = u.Avatar
		}
		sc, submitted := scoreByUser[uid]
		view.Submitted = submitted
		// Anonymity: before reveal a taster only sees their own numbers.
		if submitted && (revealed || uid == viewerID) {
			view.AromaScore = sc.AromaScore
			view.AcidityScore = sc.AcidityScore
			view.BodyScore = sc.BodyScore
			view.OverallScore = sc.OverallScore
		}
		if revealed && submitted && averages != nil {
			view.AromaOutlier = math.Abs(sc.AromaScore-averages.Aroma) > constants.BlindOutlierThreshold
			view.AcidityOutlier = math.Abs(sc.AcidityScore-averages.Acidity) > constants.BlindOutlierThreshold
			view.BodyOutlier = math.Abs(sc.BodyScore-averages.Body) > constants.BlindOutlierThreshold
			view.OverallOutlier = math.Abs(sc.OverallScore-averages.Overall) > constants.BlindOutlierThreshold
		}
		views = append(views, view)
	}

	out := &dto.BlindSessionView{
		ID:             session.ID,
		HostID:         session.HostID,
		CoffeeBeanID:   session.CoffeeBeanID,
		CoffeeBeanName: session.CoffeeBeanName,
		Status:         session.Status,
		StatusText:     util.BlindStatusText(session.Status),
		CreatedAt:      util.FormatDateTime(session.CreatedAt),
		ParticipantIDs: ids,
		SubmittedCount: len(scores),
		IsHost:         isHost,
		IsParticipant:  participantSet[viewerID],
		MySubmitted:    false,
		Scores:         views,
	}
	if h, ok := userMap[session.HostID]; ok {
		out.HostName = h.Username
	}
	if session.RevealedAt != nil {
		out.RevealedAt = util.FormatDateTime(*session.RevealedAt)
	}
	if _, ok := scoreByUser[viewerID]; ok {
		out.MySubmitted = true
	}
	if revealed {
		out.Averages = averages
	}
	s.logger.Info(fmt.Sprintf(constants.LogBlindGetSuccess, sessionID, viewerID))
	return out, nil
}

// List returns sessions, optionally restricted to the viewer's own rounds.
func (s *BlindTastingService) List(viewerID uint, involvedOnly bool, page, pageSize int) ([]dto.BlindSessionBrief, int64, error) {
	sessions, total, err := s.repo.ListSessions(viewerID, involvedOnly, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("blind list: %w", err)
	}
	hostIDs := make([]uint, 0, len(sessions))
	for _, sess := range sessions {
		hostIDs = append(hostIDs, sess.HostID)
	}
	userMap := map[uint]model.User{}
	if len(hostIDs) > 0 {
		users, err := s.userRepo.FindByIDs(hostIDs)
		if err != nil {
			return nil, 0, fmt.Errorf("blind list users: %w", err)
		}
		for _, u := range users {
			userMap[u.ID] = u
		}
	}
	briefs := make([]dto.BlindSessionBrief, 0, len(sessions))
	for _, sess := range sessions {
		participants, err := s.repo.ListParticipants(sess.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("blind list participants: %w", err)
		}
		scores, err := s.repo.ListScores(sess.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("blind list scores: %w", err)
		}
		brief := dto.BlindSessionBrief{
			ID:               sess.ID,
			HostID:           sess.HostID,
			CoffeeBeanID:     sess.CoffeeBeanID,
			CoffeeBeanName:   sess.CoffeeBeanName,
			Status:           sess.Status,
			StatusText:       util.BlindStatusText(sess.Status),
			SubmittedCount:   len(scores),
			ParticipantCount: len(participants),
			CreatedAt:        util.FormatDateTime(sess.CreatedAt),
		}
		if h, ok := userMap[sess.HostID]; ok {
			brief.HostName = h.Username
		}
		briefs = append(briefs, brief)
	}
	scope := "all"
	if involvedOnly {
		scope = "mine"
	}
	s.logger.Info(fmt.Sprintf(constants.LogBlindListSuccess, scope, viewerID), "total", total)
	return briefs, total, nil
}

// SearchUsers returns compact user rows for the participant picker.
func (s *BlindTastingService) SearchUsers(viewerID uint, keyword string) ([]dto.UserBriefView, error) {
	users, err := s.userRepo.SearchUsers(keyword, viewerID, 10)
	if err != nil {
		return nil, fmt.Errorf("blind search users: %w", err)
	}
	briefs := make([]dto.UserBriefView, 0, len(users))
	for _, u := range users {
		briefs = append(briefs, dto.UserBriefView{ID: u.ID, Username: u.Username, Avatar: u.Avatar})
	}
	return briefs, nil
}

func (s *BlindTastingService) isParticipant(sessionID, userID uint) bool {
	participants, err := s.repo.ListParticipants(sessionID)
	if err != nil {
		return false
	}
	for _, p := range participants {
		if p.UserID == userID {
			return true
		}
	}
	return false
}

func (s *BlindTastingService) loadUsers(ids []uint) (map[uint]model.User, error) {
	uniq := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id != 0 {
			uniq[id] = struct{}{}
		}
	}
	idList := make([]uint, 0, len(uniq))
	for id := range uniq {
		idList = append(idList, id)
	}
	users, err := s.userRepo.FindByIDs(idList)
	if err != nil {
		return nil, fmt.Errorf("blind load users: %w", err)
	}
	m := make(map[uint]model.User, len(users))
	for _, u := range users {
		m[u.ID] = u
	}
	return m, nil
}

// computeBlindAverages calculates the four dimension means. Returns nil when
// no score exists. Means are rounded to two decimals for display.
func computeBlindAverages(scores []model.BlindScore) *dto.BlindAverages {
	if len(scores) == 0 {
		return nil
	}
	var aroma, acidity, body, overall float64
	for _, sc := range scores {
		aroma += sc.AromaScore
		acidity += sc.AcidityScore
		body += sc.BodyScore
		overall += sc.OverallScore
	}
	n := float64(len(scores))
	return &dto.BlindAverages{
		Aroma:   roundScore(aroma / n),
		Acidity: roundScore(acidity / n),
		Body:    roundScore(body / n),
		Overall: roundScore(overall / n),
	}
}

func roundScore(v float64) float64 {
	return math.Round(v*100) / 100
}
