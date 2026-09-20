package model

import "time"

// BlindTastingSession is a blind cupping round hosted by one user
// for exactly three participants.
type BlindTastingSession struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	HostID         uint       `gorm:"index;not null" json:"host_id"`
	CoffeeBeanID   uint       `gorm:"index;not null" json:"coffee_bean_id"`
	CoffeeBeanName string     `gorm:"size:128;not null" json:"coffee_bean_name"`
	Status         string     `gorm:"size:16;index;not null;default:ongoing" json:"status"`
	RevealedAt     *time.Time `json:"revealed_at"`
	CreatedAt      time.Time  `json:"created_at"`
}
