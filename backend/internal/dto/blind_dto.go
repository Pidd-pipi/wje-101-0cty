package dto

// BlindCreateRequest starts a blind cupping round: one bean plus exactly
// three distinct participants (the host is not allowed to self-join).
type BlindCreateRequest struct {
	CoffeeBeanID   uint   `json:"coffee_bean_id" binding:"required"`
	ParticipantIDs []uint `json:"participant_ids" binding:"required"`
}

// BlindScoreSubmitRequest is a participant's single submission.
type BlindScoreSubmitRequest struct {
	AromaScore   float64 `json:"aroma_score"`
	AcidityScore float64 `json:"acidity_score"`
	BodyScore    float64 `json:"body_score"`
	OverallScore float64 `json:"overall_score"`
}

// BlindScoreView is one taster's score. During the ongoing phase only the
// requesting participant's own row carries numeric values; other rows are
// masked via the submitted flag. Outlier flags appear after reveal.
type BlindScoreView struct {
	UserID         uint    `json:"user_id"`
	Username       string  `json:"username"`
	Avatar         string  `json:"avatar"`
	Submitted      bool    `json:"submitted"`
	AromaScore     float64 `json:"aroma_score,omitempty"`
	AcidityScore   float64 `json:"acidity_score,omitempty"`
	BodyScore      float64 `json:"body_score,omitempty"`
	OverallScore   float64 `json:"overall_score,omitempty"`
	AromaOutlier   bool    `json:"aroma_outlier,omitempty"`
	AcidityOutlier bool    `json:"acidity_outlier,omitempty"`
	BodyOutlier    bool    `json:"body_outlier,omitempty"`
	OverallOutlier bool    `json:"overall_outlier,omitempty"`
}

// BlindAverages is the per-dimension mean computed at reveal time.
type BlindAverages struct {
	Aroma   float64 `json:"aroma"`
	Acidity float64 `json:"acidity"`
	Body    float64 `json:"body"`
	Overall float64 `json:"overall"`
}

// BlindSessionView is the detail payload of a blind cupping round.
type BlindSessionView struct {
	ID             uint             `json:"id"`
	HostID         uint             `json:"host_id"`
	HostName       string           `json:"host_name"`
	CoffeeBeanID   uint             `json:"coffee_bean_id"`
	CoffeeBeanName string           `json:"coffee_bean_name"`
	Status         string           `json:"status"`
	StatusText     string           `json:"status_text"`
	RevealedAt     string           `json:"revealed_at,omitempty"`
	CreatedAt      string           `json:"created_at"`
	ParticipantIDs []uint           `json:"participant_ids"`
	SubmittedCount int              `json:"submitted_count"`
	IsHost         bool             `json:"is_host"`
	IsParticipant  bool             `json:"is_participant"`
	MySubmitted    bool             `json:"my_submitted"`
	Scores         []BlindScoreView `json:"scores"`
	Averages       *BlindAverages   `json:"averages,omitempty"`
}

// BlindSessionBrief is the list-page payload.
type BlindSessionBrief struct {
	ID               uint   `json:"id"`
	HostID           uint   `json:"host_id"`
	HostName         string `json:"host_name"`
	CoffeeBeanID     uint   `json:"coffee_bean_id"`
	CoffeeBeanName   string `json:"coffee_bean_name"`
	Status           string `json:"status"`
	StatusText       string `json:"status_text"`
	SubmittedCount   int    `json:"submitted_count"`
	ParticipantCount int    `json:"participant_count"`
	CreatedAt        string `json:"created_at"`
}

// UserBriefView is the compact user shape returned by the picker search.
type UserBriefView struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}
