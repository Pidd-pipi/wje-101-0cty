package repository

import (
	"gorm.io/gorm"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
)

// BlindCuppingRepository handles blind cupping session persistence.
type BlindCuppingRepository struct{ db *gorm.DB }

// NewBlindCuppingRepository creates the repository.
func NewBlindCuppingRepository(db *gorm.DB) *BlindCuppingRepository {
	return &BlindCuppingRepository{db: db}
}

// Create inserts a session.
func (r *BlindCuppingRepository) Create(c *model.BlindCupping) error {
	return translate(r.db.Create(c).Error)
}

// FindByID locates a session by id.
func (r *BlindCuppingRepository) FindByID(id uint) (*model.BlindCupping, error) {
	var c model.BlindCupping
	if err := translate(r.db.First(&c, id).Error); err != nil {
		return nil, err
	}
	return &c, nil
}

// FindByIDLocked locates a session and takes a row lock (SELECT ... FOR UPDATE).
// It must be called inside a transaction; the lock serializes concurrent
// submissions and reveals against the same session.
func (r *BlindCuppingRepository) FindByIDLocked(id uint) (*model.BlindCupping, error) {
	var c model.BlindCupping
	if err := translate(r.db.Clauses(lockingForUpdate).First(&c, id).Error); err != nil {
		return nil, err
	}
	return &c, nil
}

// Update persists a session.
func (r *BlindCuppingRepository) Update(c *model.BlindCupping) error {
	return translate(r.db.Save(c).Error)
}

// ListForUser returns sessions the user organizes or participates in, newest first.
func (r *BlindCuppingRepository) ListForUser(userID uint) ([]model.BlindCupping, error) {
	var items []model.BlindCupping
	sub := r.db.Model(&model.CuppingParticipant{}).
		Select("cupping_id").Where("user_id = ?", userID)
	if err := translate(r.db.
		Where("organizer_id = ? OR id IN (?)", userID, sub).
		Order("id DESC").Find(&items).Error); err != nil {
		return nil, err
	}
	return items, nil
}

// RunInTx executes fn inside a database transaction.
func (r *BlindCuppingRepository) RunInTx(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}
