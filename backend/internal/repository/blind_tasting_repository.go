package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
)

// Extra sentinel errors for blind cupping state transitions.
var (
	// ErrAlreadyRevealed means a score arrived after the session was revealed.
	ErrAlreadyRevealed = errors.New("blind tasting session already revealed")
	// ErrNotAllSubmitted means the host tried to reveal before all tasters scored.
	ErrNotAllSubmitted = errors.New("blind tasting session not ready: missing submissions")
)

// BlindTastingRepository persists blind cupping sessions, participants and scores.
type BlindTastingRepository struct{ db *gorm.DB }

// NewBlindTastingRepository creates the repository.
func NewBlindTastingRepository(db *gorm.DB) *BlindTastingRepository {
	return &BlindTastingRepository{db: db}
}

// Create inserts a session together with its participants in one transaction.
func (r *BlindTastingRepository) Create(session *model.BlindTastingSession, participantIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := translate(tx.Create(session).Error); err != nil {
			return err
		}
		participants := make([]model.BlindTastingParticipant, 0, len(participantIDs))
		for _, uid := range participantIDs {
			participants = append(participants, model.BlindTastingParticipant{SessionID: session.ID, UserID: uid})
		}
		return translate(tx.Create(&participants).Error)
	})
}

// FindSession locates a session by id.
func (r *BlindTastingRepository) FindSession(id uint) (*model.BlindTastingSession, error) {
	var s model.BlindTastingSession
	if err := translate(r.db.First(&s, id).Error); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListParticipants returns the participant rows of a session.
func (r *BlindTastingRepository) ListParticipants(sessionID uint) ([]model.BlindTastingParticipant, error) {
	var items []model.BlindTastingParticipant
	if err := r.db.Where("session_id = ?", sessionID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListScores returns every submitted score of a session.
func (r *BlindTastingRepository) ListScores(sessionID uint) ([]model.BlindScore, error) {
	var items []model.BlindScore
	if err := r.db.Where("session_id = ?", sessionID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// FindScore locates one taster's score of a session.
func (r *BlindTastingRepository) FindScore(sessionID, userID uint) (*model.BlindScore, error) {
	var sc model.BlindScore
	if err := translate(r.db.Where("session_id = ? AND user_id = ?", sessionID, userID).First(&sc).Error); err != nil {
		return nil, err
	}
	return &sc, nil
}

// CountScores returns how many scores have been submitted for a session.
func (r *BlindTastingRepository) CountScores(tx *gorm.DB, sessionID uint) (int64, error) {
	var total int64
	if err := tx.Model(&model.BlindScore{}).Where("session_id = ?", sessionID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// CountParticipants returns how many tasters the session has.
func (r *BlindTastingRepository) CountParticipants(tx *gorm.DB, sessionID uint) (int64, error) {
	var total int64
	if err := tx.Model(&model.BlindTastingParticipant{}).Where("session_id = ?", sessionID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// lockSession returns a query that row-locks the sessions table on databases
// that support SELECT ... FOR UPDATE (PostgreSQL). Other dialects (e.g. SQLite
// in tests) rely on transaction serialization instead.
func (r *BlindTastingRepository) lockSession(tx *gorm.DB) *gorm.DB {
	if r.db.Dialector.Name() != "postgres" {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

// SubmitScore atomically inserts a taster's score. The session row is locked
// (SELECT ... FOR UPDATE) so concurrent submissions cannot race each other or
// a reveal. The unique index uniq_blind_score_session_user additionally rejects
// duplicate/parallel submissions as ErrDuplicate.
func (r *BlindTastingRepository) SubmitScore(score *model.BlindScore) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var session model.BlindTastingSession
		if err := translate(r.lockSession(tx).First(&session, score.SessionID).Error); err != nil {
			return err
		}
		if session.Status == "revealed" {
			return ErrAlreadyRevealed
		}
		var participant model.BlindTastingParticipant
		if err := translate(tx.Where("session_id = ? AND user_id = ?", score.SessionID, score.UserID).First(&participant).Error); err != nil {
			return err
		}
		if err := translate(tx.Create(score).Error); err != nil {
			return err
		}
		return nil
	})
}

// Reveal atomically flips a session to revealed. It locks the session row so a
// parallel reveal or score submission cannot interleave, and only succeeds
// when every participant has submitted.
func (r *BlindTastingRepository) Reveal(sessionID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var session model.BlindTastingSession
		if err := translate(r.lockSession(tx).First(&session, sessionID).Error); err != nil {
			return err
		}
		if session.Status == "revealed" {
			return ErrAlreadyRevealed
		}
		participantCount, err := r.CountParticipants(tx, sessionID)
		if err != nil {
			return err
		}
		scoreCount, err := r.CountScores(tx, sessionID)
		if err != nil {
			return err
		}
		if scoreCount < participantCount {
			return ErrNotAllSubmitted
		}
		return translate(tx.Model(&model.BlindTastingSession{}).Where("id = ?", sessionID).
			Updates(map[string]interface{}{"status": "revealed", "revealed_at": time.Now()}).Error)
	})
}

// ListSessions returns sessions in newest-first order, optionally restricted
// to those where the user is host or participant.
func (r *BlindTastingRepository) ListSessions(userID uint, involved bool, page, pageSize int) ([]model.BlindTastingSession, int64, error) {
	var items []model.BlindTastingSession
	var total int64
	q := r.db.Model(&model.BlindTastingSession{})
	if involved {
		q = q.Where("host_id = ? OR id IN (?)", userID,
			r.db.Model(&model.BlindTastingParticipant{}).Select("session_id").Where("user_id = ?", userID))
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
