package repository

import (
	"gorm.io/gorm"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
)

// CuppingParticipantRepository handles cupping participant persistence.
type CuppingParticipantRepository struct{ db *gorm.DB }

// NewCuppingParticipantRepository creates the repository.
func NewCuppingParticipantRepository(db *gorm.DB) *CuppingParticipantRepository {
	return &CuppingParticipantRepository{db: db}
}

// CreateBatch inserts all participants of a session.
func (r *CuppingParticipantRepository) CreateBatch(items []model.CuppingParticipant) error {
	if len(items) == 0 {
		return nil
	}
	return translate(r.db.Create(&items).Error)
}

// ListByCupping returns the participants of a session ordered by user id.
func (r *CuppingParticipantRepository) ListByCupping(cuppingID uint) ([]model.CuppingParticipant, error) {
	var items []model.CuppingParticipant
	if err := translate(r.db.Where("cupping_id = ?", cuppingID).
		Order("user_id ASC").Find(&items).Error); err != nil {
		return nil, err
	}
	return items, nil
}
