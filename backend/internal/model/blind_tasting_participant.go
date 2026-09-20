package model

import "time"

// BlindTastingParticipant links a user to a blind cupping session as a taster.
// A user can participate in a session at most once.
type BlindTastingParticipant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID uint      `gorm:"uniqueIndex:uniq_blind_session_user;not null" json:"session_id"`
	UserID    uint      `gorm:"uniqueIndex:uniq_blind_session_user;not null" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
