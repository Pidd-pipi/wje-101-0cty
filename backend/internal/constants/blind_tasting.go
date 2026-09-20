package constants

// BlindTastingStatus enumerates blind cupping session states.
const (
	BlindStatusOngoing  = "ongoing"  // 开评中：匿名提交阶段
	BlindStatusRevealed = "revealed" // 已揭晓：评分与均值公开
)

const (
	// BlindParticipantCount is the required number of tasters per session.
	BlindParticipantCount = 3
	// BlindOutlierThreshold marks a score as outlier when its deviation
	// from the dimension average exceeds this value.
	BlindOutlierThreshold = 1.5
	// BlindMinScore / BlindMaxScore bound every submitted dimension.
	BlindMinScore = 0.0
	BlindMaxScore = 10.0
)

// ValidBlindStatuses returns all accepted blind cupping states.
func ValidBlindStatuses() []string {
	return []string{BlindStatusOngoing, BlindStatusRevealed}
}

// IsValidBlindStatus reports whether a status is known.
func IsValidBlindStatus(s string) bool {
	for _, v := range ValidBlindStatuses() {
		if v == s {
			return true
		}
	}
	return false
}

// IsValidBlindScore reports whether a dimension score is inside [0, 10].
func IsValidBlindScore(v float64) bool {
	return v >= BlindMinScore && v <= BlindMaxScore
}
