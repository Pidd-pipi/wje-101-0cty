package service

import (
	"testing"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/model"
)

func TestComputeAverages(t *testing.T) {
	scores := []model.BlindScore{
		{AromaScore: 8.0, AcidityScore: 7.0, BodyScore: 6.0, OverallScore: 8.0},
		{AromaScore: 9.0, AcidityScore: 8.0, BodyScore: 7.0, OverallScore: 8.0},
		{AromaScore: 7.0, AcidityScore: 6.0, BodyScore: 8.0, OverallScore: 8.0},
	}
	got := computeAverages(scores)
	want := dimensionAverages{aroma: 8.0, acidity: 7.0, body: 7.0, overall: 8.0}
	if got != want {
		t.Fatalf("computeAverages = %+v, want %+v", got, want)
	}
}

func TestComputeAveragesRoundsToOneDecimal(t *testing.T) {
	scores := []model.BlindScore{
		{AromaScore: 8.0, AcidityScore: 0, BodyScore: 0, OverallScore: 0},
		{AromaScore: 8.0, AcidityScore: 0, BodyScore: 0, OverallScore: 0},
		{AromaScore: 8.2, AcidityScore: 0, BodyScore: 0, OverallScore: 0},
	}
	got := computeAverages(scores)
	if got.aroma != 8.1 {
		t.Fatalf("rounded aroma avg = %v, want 8.1", got.aroma)
	}
}

func TestIsOutlierThreshold(t *testing.T) {
	cases := []struct {
		name  string
		score float64
		avg   float64
		want  bool
	}{
		{"exactly at 1.5 is not outlier", 6.5, 8.0, false},
		{"just beyond 1.5 is outlier", 6.4, 8.0, true},
		{"at average is not outlier", 8.0, 8.0, false},
		{"positive deviation beyond 1.5", 9.6, 8.0, true},
		{"small deviation", 7.0, 8.0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOutlier(tc.score, tc.avg); got != tc.want {
				t.Fatalf("isOutlier(%v,%v) = %v, want %v", tc.score, tc.avg, got, tc.want)
			}
		})
	}
}
