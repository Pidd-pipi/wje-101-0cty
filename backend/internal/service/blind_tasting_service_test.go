package service

import (
	"testing"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/dto"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
)

func TestValidateBlindParticipants(t *testing.T) {
	cases := []struct {
		name    string
		hostID  uint
		ids     []uint
		wantErr bool
	}{
		{"exactly three distinct", 1, []uint{2, 3, 4}, false},
		{"too few", 1, []uint{2, 3}, true},
		{"too many", 1, []uint{2, 3, 4, 5}, true},
		{"empty", 1, nil, true},
		{"duplicate participant", 1, []uint{2, 2, 3}, true},
		{"host joins himself", 1, []uint{1, 2, 3}, true},
		{"zero id", 1, []uint{0, 2, 3}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBlindParticipants(tc.hostID, tc.ids)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}

func TestComputeBlindAverages(t *testing.T) {
	scores := []model.BlindScore{
		{AromaScore: 8, AcidityScore: 7, BodyScore: 6, OverallScore: 7},
		{AromaScore: 9, AcidityScore: 8, BodyScore: 7, OverallScore: 8},
		{AromaScore: 8.5, AcidityScore: 7.5, BodyScore: 6.5, OverallScore: 7.5},
	}
	avg := computeBlindAverages(scores)
	if avg == nil {
		t.Fatal("expected averages")
	}
	if avg.Aroma != 8.5 || avg.Acidity != 7.5 || avg.Body != 6.5 || avg.Overall != 7.5 {
		t.Fatalf("unexpected averages: %+v", avg)
	}

	// Outlier rule: deviation strictly greater than 1.5 from the average.
	outlier := model.BlindScore{AromaScore: 6, AcidityScore: 7, BodyScore: 6, OverallScore: 7}
	if got := absf(outlier.AromaScore - avg.Aroma); got <= constants.BlindOutlierThreshold {
		t.Fatalf("aroma deviation %v should exceed %v", got, constants.BlindOutlierThreshold)
	}
	boundary := model.BlindScore{AromaScore: 7} // |7 - 8.5| == 1.5, not an outlier
	if absf(boundary.AromaScore-avg.Aroma) > constants.BlindOutlierThreshold {
		t.Fatal("a deviation of exactly 1.5 must not be marked outlier")
	}
}

func TestComputeBlindAveragesEmpty(t *testing.T) {
	if computeBlindAverages(nil) != nil {
		t.Fatal("expected nil averages without scores")
	}
	var _ dto.BlindAverages // keep dto import meaningful if cases expand
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
