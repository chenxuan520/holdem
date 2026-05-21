package match

import "testing"

func TestEvaluateBestHand(t *testing.T) {
	tests := []struct {
		name     string
		cards    []string
		category int
		label    string
	}{
		{name: "straight flush", cards: []string{"As", "Ks", "Qs", "Js", "Ts", "2d", "3c"}, category: 8, label: "同花顺"},
		{name: "full house", cards: []string{"Ah", "Ad", "Ac", "Ks", "Kd", "2c", "3d"}, category: 6, label: "葫芦"},
		{name: "two pair", cards: []string{"Ah", "Ad", "Ks", "Kd", "2c", "3d", "4h"}, category: 2, label: "两对"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rank, label, err := evaluateBestHand(tt.cards)
			if err != nil {
				t.Fatalf("evaluateBestHand returned error: %v", err)
			}
			if rank.Category != tt.category {
				t.Fatalf("expected category %d, got %d", tt.category, rank.Category)
			}
			if label != tt.label {
				t.Fatalf("expected label %q, got %q", tt.label, label)
			}
		})
	}
}
