package dto

import "time"

// CuppingCreateRequest starts a blind cupping session with one bean and three participants.
// Exactly three distinct participants is enforced in the service (422) so the
// error message is explicit and no data is written.
type CuppingCreateRequest struct {
	CoffeeBeanID   uint   `json:"coffee_bean_id" binding:"required"`
	ParticipantIDs []uint `json:"participant_ids" binding:"required,min=1"`
}

// CuppingScoreRequest is a participant's one-time submission.
type CuppingScoreRequest struct {
	AromaScore   float64 `json:"aroma_score" binding:"gte=0,lte=10"`
	AcidityScore float64 `json:"acidity_score" binding:"gte=0,lte=10"`
	BodyScore    float64 `json:"body_score" binding:"gte=0,lte=10"`
	OverallScore float64 `json:"overall_score" binding:"gte=0,lte=10"`
}

// CuppingUserView is the compact user shape embedded in cupping views.
type CuppingUserView struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// CuppingBeanView is the compact bean shape embedded in cupping views.
type CuppingBeanView struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Origin        string `json:"origin"`
	ProcessMethod string `json:"process_method"`
}

// CuppingParticipantView shows who was invited and whether they submitted.
type CuppingParticipantView struct {
	UserID    uint   `json:"user_id"`
	Username  string `json:"username"`
	Avatar    string `json:"avatar"`
	Submitted bool   `json:"submitted"`
}

// CuppingScoreView is a revealed submission with per-dimension outlier flags.
type CuppingScoreView struct {
	UserID         uint    `json:"user_id"`
	Username       string  `json:"username"`
	AromaScore     float64 `json:"aroma_score"`
	AcidityScore   float64 `json:"acidity_score"`
	BodyScore      float64 `json:"body_score"`
	OverallScore   float64 `json:"overall_score"`
	OutlierAroma   bool    `json:"outlier_aroma"`
	OutlierAcidity bool    `json:"outlier_acidity"`
	OutlierBody    bool    `json:"outlier_body"`
	OutlierOverall bool    `json:"outlier_overall"`
}

// CuppingView is the full session representation.
type CuppingView struct {
	ID             uint                       `json:"id"`
	Status         string                     `json:"status"`
	Organizer      CuppingUserView            `json:"organizer"`
	CoffeeBean     CuppingBeanView            `json:"coffee_bean"`
	Participants   []CuppingParticipantView   `json:"participants"`
	SubmittedCount int                        `json:"submitted_count"`
	Scores         []CuppingScoreView         `json:"scores"`
	AvgAroma       float64                    `json:"avg_aroma"`
	AvgAcidity     float64                    `json:"avg_acidity"`
	AvgBody        float64                    `json:"avg_body"`
	AvgOverall     float64                    `json:"avg_overall"`
	CanSubmit      bool                       `json:"can_submit"`
	CanReveal      bool                       `json:"can_reveal"`
	RevealedAt     *time.Time                 `json:"revealed_at"`
	CreatedAt      time.Time                  `json:"created_at"`
}

// CuppingListItemView is a session row in the list view.
type CuppingListItemView struct {
	ID             uint            `json:"id"`
	Status         string          `json:"status"`
	Organizer      CuppingUserView `json:"organizer"`
	CoffeeBean     CuppingBeanView `json:"coffee_bean"`
	SubmittedCount int             `json:"submitted_count"`
	ParticipantTotal int           `json:"participant_total"`
	CreatedAt      time.Time       `json:"created_at"`
	RevealedAt     *time.Time      `json:"revealed_at"`
}
