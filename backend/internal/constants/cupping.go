package constants

// Blind cupping session status.
const (
	CuppingCollecting = "collecting" // 收集中：尚未揭晓，评分互不可见
	CuppingRevealed   = "revealed"   // 已揭晓：平均分与离群标记公开
)

// CuppingParticipantCount is the fixed number of participants per session.
const CuppingParticipantCount = 3

// CuppingOutlierThreshold is the absolute deviation (in points) from the
// dimension average that flags a score as an outlier. Deviation strictly
// greater than 1.5 is outlier.
const CuppingOutlierThreshold = 1.5

// Cupping score bounds (0-10, matching tasting note scores).
const (
	CuppingMinScore = 0.0
	CuppingMaxScore = 10.0
)

// Score dimension keys shared by backend, DTO, logs and frontend constants.
const (
	DimensionAroma   = "aroma"
	DimensionAcidity = "acidity"
	DimensionBody    = "body"
	DimensionOverall = "overall"
)

// CuppingDimensions returns all scored dimensions in display order.
func CuppingDimensions() []string {
	return []string{DimensionAroma, DimensionAcidity, DimensionBody, DimensionOverall}
}

// IsValidCuppingStatus reports whether a session status is known.
func IsValidCuppingStatus(s string) bool {
	return s == CuppingCollecting || s == CuppingRevealed
}

// IsValidCuppingScore reports whether a dimension score is inside [0, 10].
func IsValidCuppingScore(v float64) bool {
	return v >= CuppingMinScore && v <= CuppingMaxScore
}
