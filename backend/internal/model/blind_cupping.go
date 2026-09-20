package model

import "time"

// BlindCupping is a blind cupping session. The organizer picks one coffee
// bean and exactly three participants. Scores stay hidden (collecting)
// until the organizer reveals them, at which point dimension averages and
// outlier flags are computed.
type BlindCupping struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	OrganizerID  uint      `gorm:"index;not null" json:"organizer_id"`
	CoffeeBeanID uint      `gorm:"index;not null" json:"coffee_bean_id"`
	Status       string    `gorm:"size:16;index;not null;default:collecting" json:"status"`
	AvgAroma     float64   `json:"avg_aroma"`
	AvgAcidity   float64   `json:"avg_acidity"`
	AvgBody      float64   `json:"avg_body"`
	AvgOverall   float64   `json:"avg_overall"`
	RevealedAt   *time.Time `json:"revealed_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// CuppingParticipant is one of the three invited users of a session.
type CuppingParticipant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CuppingID uint      `gorm:"index:idx_cupping_participant,unique;not null" json:"cupping_id"`
	UserID    uint      `gorm:"index:idx_cupping_participant,unique;not null" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BlindScore is a participant's single submission. The unique pair
// (cupping_id, user_id) enforces one submission per participant at the
// database level, which also rejects concurrent duplicate inserts.
type BlindScore struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CuppingID   uint      `gorm:"index:idx_blindscore_cupping_user,unique;not null" json:"cupping_id"`
	UserID       uint      `gorm:"index:idx_blindscore_cupping_user,unique;not null" json:"user_id"`
	AromaScore   float64   `json:"aroma_score"`
	AcidityScore float64   `json:"acidity_score"`
	BodyScore    float64   `json:"body_score"`
	OverallScore float64   `json:"overall_score"`
	CreatedAt    time.Time `json:"created_at"`
}
