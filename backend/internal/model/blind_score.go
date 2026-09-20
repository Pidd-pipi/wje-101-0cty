package model

import "time"

// BlindScore is a participant's one-shot submission for a blind cupping session.
// Aromas/acidity/body/overall are all scored out of 10.
type BlindScore struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SessionID    uint      `gorm:"uniqueIndex:uniq_blind_score_session_user;not null" json:"session_id"`
	UserID       uint      `gorm:"uniqueIndex:uniq_blind_score_session_user;not null" json:"user_id"`
	AromaScore   float64   `json:"aroma_score"`
	AcidityScore float64   `json:"acidity_score"`
	BodyScore    float64   `json:"body_score"`
	OverallScore float64   `json:"overall_score"`
	CreatedAt    time.Time `json:"created_at"`
}
