package cli

import (
	"encoding/json"
	"testing"
)

func TestShopperIngredientQueryStripsRecipeQuantities(t *testing.T) {
	tests := map[string]string{
		"40 g butter":                      "butter",
		"1 eschalot":                       "eschalot",
		"1 - 2 tbsp Vegetable stock paste": "Vegetable stock paste",
		"1 pinch ground cayenne pepper":    "ground cayenne pepper",
	}
	for input, want := range tests {
		if got := shopperIngredientQuery(input); got != want {
			t.Errorf("shopperIngredientQuery(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestShopperCandidateCount(t *testing.T) {
	raw := json.RawMessage(`{"results":{"products":[{"id":1},{"id":2}]}}`)
	if got := shopperCandidateCount(raw); got != 2 {
		t.Fatalf("shopperCandidateCount() = %d, want 2", got)
	}
}
