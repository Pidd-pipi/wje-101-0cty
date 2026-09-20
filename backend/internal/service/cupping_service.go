package service

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"gorm.io/gorm"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/dto"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/repository"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/util"
)

// CuppingService implements the blind cupping closed loop:
// start -> one anonymous submission per participant -> organizer reveal ->
// dimension averages and outlier flags.
type CuppingService struct {
	cuppingRepo     *repository.BlindCuppingRepository
	participantRepo *repository.CuppingParticipantRepository
	scoreRepo       *repository.BlindScoreRepository
	beanRepo        *repository.CoffeeBeanRepository
	userRepo        *repository.UserRepository
	logger          *slog.Logger
}

// NewCuppingService creates a CuppingService.
func NewCuppingService(
	cuppingRepo *repository.BlindCuppingRepository,
	participantRepo *repository.CuppingParticipantRepository,
	scoreRepo *repository.BlindScoreRepository,
	beanRepo *repository.CoffeeBeanRepository,
	userRepo *repository.UserRepository,
	logger *slog.Logger,
) *CuppingService {
	return &CuppingService{
		cuppingRepo:     cuppingRepo,
		participantRepo: participantRepo,
		scoreRepo:       scoreRepo,
		beanRepo:        beanRepo,
		userRepo:        userRepo,
		logger:          logger,
	}
}

// Create starts a session with one bean and exactly three distinct participants.
func (s *CuppingService) Create(organizerID, beanID uint, participantIDs []uint) (*dto.CuppingView, error) {
	if len(participantIDs) != constants.CuppingParticipantCount {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("BlindCupping[bean_id=%d] create failed: participant count must be %d, got %d",
				beanID, constants.CuppingParticipantCount, len(participantIDs)))
	}
	seen := make(map[uint]struct{}, constants.CuppingParticipantCount)
	for _, pid := range participantIDs {
		if pid == 0 {
			return nil, util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("BlindCupping[bean_id=%d] create failed: participant user_id is empty", beanID))
		}
		if _, ok := seen[pid]; ok {
			return nil, util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("BlindCupping[bean_id=%d] create failed: participant user_id=%d repeated", beanID, pid))
		}
		seen[pid] = struct{}{}
	}
	bean, err := s.beanRepo.FindByID(beanID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("CoffeeBean[id=%d] not found", beanID))
		}
		return nil, fmt.Errorf("cupping create bean: %w", err)
	}
	users, err := s.userRepo.FindByIDs(participantIDs)
	if err != nil {
		return nil, fmt.Errorf("cupping create users: %w", err)
	}
	if len(users) != constants.CuppingParticipantCount {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("BlindCupping[bean_id=%d] create failed: %d participants do not exist", beanID,
				constants.CuppingParticipantCount-len(users)))
	}

	c := &model.BlindCupping{OrganizerID: organizerID, CoffeeBeanID: beanID, Status: constants.CuppingCollecting}
	err = s.cuppingRepo.RunInTx(func(tx *gorm.DB) error {
		if err := repository.NewBlindCuppingRepository(tx).Create(c); err != nil {
			return fmt.Errorf("cupping create session: %w", err)
		}
		parts := make([]model.CuppingParticipant, 0, constants.CuppingParticipantCount)
		for _, pid := range participantIDs {
			parts = append(parts, model.CuppingParticipant{CuppingID: c.ID, UserID: pid})
		}
		if err := repository.NewCuppingParticipantRepository(tx).CreateBatch(parts); err != nil {
			return fmt.Errorf("cupping create participants: %w", err)
		}
		return nil
	})
	if err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogCuppingCreateFailed, beanID, organizerID), "error", err)
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogCuppingCreateSuccess, c.ID, beanID, organizerID), "bean", bean.Name)
	return s.Get(c.ID, organizerID)
}

// Submit records the caller's single submission. It runs in a locked
// transaction so that duplicate/concurrent submissions fail without
// changing existing data.
func (s *CuppingService) Submit(cuppingID, userID uint, req dto.CuppingScoreRequest) (*dto.CuppingView, error) {
	for _, v := range []float64{req.AromaScore, req.AcidityScore, req.BodyScore, req.OverallScore} {
		if !constants.IsValidCuppingScore(v) {
			return nil, util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("BlindCupping[id=%d] submit failed: score %.2f out of range [%.0f,%.0f]",
					cuppingID, v, constants.CuppingMinScore, constants.CuppingMaxScore))
		}
	}

	err := s.cuppingRepo.RunInTx(func(tx *gorm.DB) error {
		cRepo := repository.NewBlindCuppingRepository(tx)
		c, err := cRepo.FindByIDLocked(cuppingID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound,
					fmt.Sprintf("BlindCupping[id=%d] not found", cuppingID))
			}
			return fmt.Errorf("cupping submit find: %w", err)
		}
		if c.Status == constants.CuppingRevealed {
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("BlindCupping[id=%d] submit failed: session already revealed", cuppingID))
		}
		isParticipant := false
		parts, err := repository.NewCuppingParticipantRepository(tx).ListByCupping(cuppingID)
		if err != nil {
			return fmt.Errorf("cupping submit participants: %w", err)
		}
		for _, p := range parts {
			if p.UserID == userID {
				isParticipant = true
				break
			}
		}
		if !isParticipant {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("BlindCupping[id=%d] submit failed: user_id=%d is not a participant", cuppingID, userID))
		}
		scoreRepo := repository.NewBlindScoreRepository(tx)
		if existing, ferr := scoreRepo.Find(cuppingID, userID); ferr == nil {
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("BlindScore[cupping_id=%d user_id=%d] submit failed: already submitted (score_id=%d)",
					cuppingID, userID, existing.ID))
		} else if !errors.Is(ferr, repository.ErrNotFound) {
			return fmt.Errorf("cupping submit find score: %w", ferr)
		}
		sc := &model.BlindScore{
			CuppingID:   cuppingID,
			UserID:      userID,
			AromaScore:  req.AromaScore,
			AcidityScore: req.AcidityScore,
			BodyScore:   req.BodyScore,
			OverallScore: req.OverallScore,
		}
		if err := scoreRepo.Create(sc); err != nil {
			if errors.Is(err, repository.ErrDuplicate) {
				return util.NewAppError(409, constants.CodeConflict,
					fmt.Sprintf("BlindScore[cupping_id=%d user_id=%d] submit failed: concurrent duplicate submission",
						cuppingID, userID))
			}
			return fmt.Errorf("cupping submit create: %w", err)
		}
		return nil
	})
	if err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogCuppingSubmitFailed, cuppingID, userID), "error", err)
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogCuppingSubmitSuccess, cuppingID, userID))
	return s.Get(cuppingID, userID)
}

// Reveal closes the collection phase and computes dimension averages.
// It fails (and leaves data untouched) when the caller is not the
// organizer, not everyone has submitted, or the session was already revealed.
func (s *CuppingService) Reveal(cuppingID, userID uint) (*dto.CuppingView, error) {
	err := s.cuppingRepo.RunInTx(func(tx *gorm.DB) error {
		cRepo := repository.NewBlindCuppingRepository(tx)
		c, err := cRepo.FindByIDLocked(cuppingID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(404, constants.CodeNotFound,
					fmt.Sprintf("BlindCupping[id=%d] not found", cuppingID))
			}
			return fmt.Errorf("cupping reveal find: %w", err)
		}
		if c.OrganizerID != userID {
			return util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("BlindCupping[id=%d] reveal failed: user_id=%d not organizer=%d",
					cuppingID, userID, c.OrganizerID))
		}
		if c.Status == constants.CuppingRevealed {
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("BlindCupping[id=%d] reveal failed: already revealed", cuppingID))
		}
		parts, err := repository.NewCuppingParticipantRepository(tx).ListByCupping(cuppingID)
		if err != nil {
			return fmt.Errorf("cupping reveal participants: %w", err)
		}
		scores, err := repository.NewBlindScoreRepository(tx).ListByCupping(cuppingID)
		if err != nil {
			return fmt.Errorf("cupping reveal scores: %w", err)
		}
		if len(scores) != len(parts) || len(scores) != constants.CuppingParticipantCount {
			return util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("BlindCupping[id=%d] reveal failed: not all submitted (%d/%d)",
					cuppingID, len(scores), constants.CuppingParticipantCount))
		}
		var sumAroma, sumAcidity, sumBody, sumOverall float64
		for _, sc := range scores {
			sumAroma += sc.AromaScore
			sumAcidity += sc.AcidityScore
			sumBody += sc.BodyScore
			sumOverall += sc.OverallScore
		}
		avg := computeAverages(scores)
		c.AvgAroma = avg.aroma
		c.AvgAcidity = avg.acidity
		c.AvgBody = avg.body
		c.AvgOverall = avg.overall
		now := time.Now()
		c.RevealedAt = &now
		c.Status = constants.CuppingRevealed
		if err := cRepo.Update(c); err != nil {
			return fmt.Errorf("cupping reveal update: %w", err)
		}
		return nil
	})
	if err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogCuppingRevealFailed, cuppingID, userID), "error", err)
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(constants.LogCuppingRevealSuccess, cuppingID, userID))
	return s.Get(cuppingID, userID)
}

// Get returns the full session view. Scores are hidden until revealed.
func (s *CuppingService) Get(cuppingID, viewerID uint) (*dto.CuppingView, error) {
	c, err := s.cuppingRepo.FindByID(cuppingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound,
				fmt.Sprintf("BlindCupping[id=%d] not found", cuppingID))
		}
		return nil, fmt.Errorf("cupping get: %w", err)
	}
	return s.buildView(c, viewerID)
}

// List returns sessions the user organizes or participates in.
func (s *CuppingService) List(userID uint) ([]dto.CuppingListItemView, error) {
	items, err := s.cuppingRepo.ListForUser(userID)
	if err != nil {
		return nil, fmt.Errorf("cupping list: %w", err)
	}
	result := make([]dto.CuppingListItemView, 0, len(items))
	for i := range items {
		c := items[i]
		view, err := s.buildView(&c, userID)
		if err != nil {
			return nil, err
		}
		result = append(result, dto.CuppingListItemView{
			ID:               view.ID,
			Status:           view.Status,
			Organizer:        view.Organizer,
			CoffeeBean:       view.CoffeeBean,
			SubmittedCount:   view.SubmittedCount,
			ParticipantTotal: len(view.Participants),
			CreatedAt:        view.CreatedAt,
			RevealedAt:       view.RevealedAt,
		})
	}
	s.logger.Info(fmt.Sprintf(constants.LogCuppingListSuccess, userID, len(result)))
	return result, nil
}

// buildView assembles the session DTO, loading related bean/users and hiding
// scores before reveal.
func (s *CuppingService) buildView(c *model.BlindCupping, viewerID uint) (*dto.CuppingView, error) {
	organizer, err := s.userRepo.FindByID(c.OrganizerID)
	if err != nil {
		return nil, fmt.Errorf("cupping view organizer: %w", err)
	}
	bean, err := s.beanRepo.FindByID(c.CoffeeBeanID)
	if err != nil {
		return nil, fmt.Errorf("cupping view bean: %w", err)
	}
	parts, err := s.participantRepo.ListByCupping(c.ID)
	if err != nil {
		return nil, fmt.Errorf("cupping view participants: %w", err)
	}
	scores, err := s.scoreRepo.ListByCupping(c.ID)
	if err != nil {
		return nil, fmt.Errorf("cupping view scores: %w", err)
	}

	ids := make([]uint, 0, len(parts))
	submitted := make(map[uint]*model.BlindScore, len(scores))
	for _, sc := range scores {
		sc := sc
		submitted[sc.UserID] = &sc
	}
	for _, p := range parts {
		ids = append(ids, p.UserID)
	}
	users, err := s.userRepo.FindByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("cupping view users: %w", err)
	}
	userByID := make(map[uint]*model.User, len(users))
	for i := range users {
		u := users[i]
		userByID[u.ID] = &u
	}

	view := &dto.CuppingView{
		ID:     c.ID,
		Status: c.Status,
		Organizer: dto.CuppingUserView{ID: organizer.ID, Username: organizer.Username, Avatar: organizer.Avatar},
		CoffeeBean: dto.CuppingBeanView{
			ID: bean.ID, Name: bean.Name, Origin: bean.Origin, ProcessMethod: bean.ProcessMethod,
		},
		Participants:   make([]dto.CuppingParticipantView, 0, len(parts)),
		Scores:         make([]dto.CuppingScoreView, 0, len(scores)),
		SubmittedCount: len(scores),
		AvgAroma:       c.AvgAroma,
		AvgAcidity:     c.AvgAcidity,
		AvgBody:        c.AvgBody,
		AvgOverall:     c.AvgOverall,
		RevealedAt:     c.RevealedAt,
		CreatedAt:      c.CreatedAt,
	}

	for _, p := range parts {
		u := userByID[p.UserID]
		pv := dto.CuppingParticipantView{UserID: p.UserID}
		if u != nil {
			pv.Username = u.Username
			pv.Avatar = u.Avatar
		}
		_, pv.Submitted = submitted[p.UserID]
		view.Participants = append(view.Participants, pv)
	}

	_, viewerSubmitted := submitted[viewerID]
	viewerIsParticipant := false
	for _, p := range parts {
		if p.UserID == viewerID {
			viewerIsParticipant = true
			break
		}
	}
	view.CanSubmit = viewerIsParticipant && !viewerSubmitted && c.Status == constants.CuppingCollecting
	view.CanReveal = c.OrganizerID == viewerID && c.Status == constants.CuppingCollecting

	// Scores stay sealed until the organizer reveals the session.
	if c.Status != constants.CuppingRevealed {
		return view, nil
	}
	for _, sc := range scores {
		u := userByID[sc.UserID]
		sv := dto.CuppingScoreView{
			UserID:         sc.UserID,
			AromaScore:     sc.AromaScore,
			AcidityScore:   sc.AcidityScore,
			BodyScore:      sc.BodyScore,
			OverallScore:   sc.OverallScore,
			OutlierAroma:   isOutlier(sc.AromaScore, c.AvgAroma),
			OutlierAcidity: isOutlier(sc.AcidityScore, c.AvgAcidity),
			OutlierBody:    isOutlier(sc.BodyScore, c.AvgBody),
			OutlierOverall: isOutlier(sc.OverallScore, c.AvgOverall),
		}
		if u != nil {
			sv.Username = u.Username
		}
		view.Scores = append(view.Scores, sv)
	}
	return view, nil
}

// isOutlier flags a dimension score whose absolute deviation from the
// average strictly exceeds the 1.5-point threshold.
func isOutlier(score, avg float64) bool {
	return math.Abs(score-avg) > constants.CuppingOutlierThreshold
}

// dimensionAverages holds the four dimension averages of a revealed session.
type dimensionAverages struct {
	aroma   float64
	acidity float64
	body    float64
	overall float64
}

// computeAverages averages each dimension over the submitted scores.
func computeAverages(scores []model.BlindScore) dimensionAverages {
	if len(scores) == 0 {
		return dimensionAverages{}
	}
	var a dimensionAverages
	for _, sc := range scores {
		a.aroma += sc.AromaScore
		a.acidity += sc.AcidityScore
		a.body += sc.BodyScore
		a.overall += sc.OverallScore
	}
	n := float64(len(scores))
	a.aroma = roundOne(a.aroma / n)
	a.acidity = roundOne(a.acidity / n)
	a.body = roundOne(a.body / n)
	a.overall = roundOne(a.overall / n)
	return a
}

func roundOne(v float64) float64 {
	return math.Round(v*10) / 10
}
