package repository

import (
	"gorm.io/gorm"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
)

// BlindScoreRepository handles blind score submission persistence.
type BlindScoreRepository struct{ db *gorm.DB }

// NewBlindScoreRepository creates the repository.
func NewBlindScoreRepository(db *gorm.DB) *BlindScoreRepository {
	return &BlindScoreRepository{db: db}
}

// Create inserts a score submission. The unique index on (cupping_id,user_id)
// makes a concurrent second insert fail with ErrDuplicate.
func (r *BlindScoreRepository) Create(s *model.BlindScore) error {
	return translate(r.db.Create(s).Error)
}

// Find locates a participant's submission.
func (r *BlindScoreRepository) Find(cuppingID, userID uint) (*model.BlindScore, error) {
	var s model.BlindScore
	if err := translate(r.db.Where("cupping_id = ? AND user_id = ?", cuppingID, userID).
		First(&s).Error); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListByCupping returns all submissions of a session ordered by user id.
func (r *BlindScoreRepository) ListByCupping(cuppingID uint) ([]model.BlindScore, error) {
	var items []model.BlindScore
	if err := translate(r.db.Where("cupping_id = ?", cuppingID).
		Order("user_id ASC").Find(&items).Error); err != nil {
		return nil, err
	}
	return items, nil
}

// CountByCupping returns the number of submissions received so far.
func (r *BlindScoreRepository) CountByCupping(cuppingID uint) (int64, error) {
	var total int64
	if err := r.db.Model(&model.BlindScore{}).
		Where("cupping_id = ?", cuppingID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
