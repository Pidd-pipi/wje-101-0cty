package repository

import (
	"gorm.io/gorm"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
)

// UserRepository handles user persistence.
type UserRepository struct{ db *gorm.DB }

// NewUserRepository creates a UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

// Create inserts a user.
func (r *UserRepository) Create(u *model.User) error { return translate(r.db.Create(u).Error) }

// FindByUsername locates a user by username.
func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	var u model.User
	if err := translate(r.db.Where("username = ?", username).First(&u).Error); err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByID locates a user by id.
func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var u model.User
	if err := translate(r.db.First(&u, id).Error); err != nil {
		return nil, err
	}
	return &u, nil
}

// Update persists a user.
func (r *UserRepository) Update(u *model.User) error { return translate(r.db.Save(u).Error) }

// ExistsByID reports whether a user exists.
func (r *UserRepository) ExistsByID(id uint) (bool, error) {
	var total int64
	if err := r.db.Model(&model.User{}).Where("id = ?", id).Count(&total).Error; err != nil {
		return false, err
	}
	return total > 0, nil
}

// FindByIDs returns users matching the given ids.
func (r *UserRepository) FindByIDs(ids []uint) ([]model.User, error) {
	var users []model.User
	if len(ids) == 0 {
		return users, nil
	}
	if err := translate(r.db.Where("id IN ?", ids).Order("id ASC").Find(&users).Error); err != nil {
		return nil, err
	}
	return users, nil
}

// Search returns users whose username contains keyword, limited to limit rows.
func (r *UserRepository) Search(keyword string, limit int) ([]model.User, error) {
	var users []model.User
	q := r.db.Model(&model.User{})
	if keyword != "" {
		q = q.Where("username LIKE ?", "%"+keyword+"%")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if err := translate(q.Order("id ASC").Limit(limit).Find(&users).Error); err != nil {
		return nil, err
	}
	return users, nil
}
