package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookidoo/internal/client"
	"github.com/spf13/cobra"
)

func TestExtractCookidooRecipeHTMLReturnsEmbeddedRecipe(t *testing.T) {
	page := []byte(`<html><head><script type="application/ld+json">{"@type":"Recipe","name":"Test soup","recipeIngredient":["2 g salt"]}</script></head></html>`)
	got, err := ExtractCookidooRecipeHTML(page, "https://cookidoo.example/recipes/recipe/en-US/r123", "text/html; charset=utf-8")
	if err != nil {
		t.Fatalf("ExtractCookidooRecipeHTML() error = %v", err)
	}
	var recipe struct {
		Name        string   `json:"name"`
		Ingredients []string `json:"recipeIngredient"`
	}
	if err := json.Unmarshal(got, &recipe); err != nil {
		t.Fatalf("extracted data is not JSON: %v", err)
	}
	if recipe.Name != "Test soup" || len(recipe.Ingredients) != 1 || recipe.Ingredients[0] != "2 g salt" {
		t.Fatalf("extracted recipe = %+v", recipe)
	}
}

func TestParseCookidooRecipeRef(t *testing.T) {
	tests := []struct {
		value, locale, id string
	}{
		{value: "r68517", locale: "en-US", id: "r68517"},
		{value: "https://cookidoo.thermomix.com/recipes/recipe/pt-BR/r68517", locale: "pt-BR", id: "r68517"},
	}
	for _, tt := range tests {
		locale, id, _, err := parseCookidooRecipeRef(tt.value, "en-US")
		if err != nil {
			t.Fatalf("parseCookidooRecipeRef(%q): %v", tt.value, err)
		}
		if locale != tt.locale || id != tt.id {
			t.Fatalf("parseCookidooRecipeRef(%q) = locale %q, id %q; want %q, %q", tt.value, locale, id, tt.locale, tt.id)
		}
	}
}

func TestCookidooRecipeViewsRequireLiveDataSource(t *testing.T) {
	flags := &rootFlags{dataSource: "local"}
	run := func(*cobra.Command, *rootFlags, *client.Client, []string, string) error { return nil }
	for _, name := range []string{"details", "ingredients", "nutrition"} {
		cmd := newCookidooViewCmd(flags, name, "test", run)
		if err := validateDataSourceStrategy(flags, commandDataSourceAnnotation(cmd)); err == nil {
			t.Errorf("%s accepted --data-source local", name)
		}
	}
	plan := newCookidooPlanCmd(flags)
	if err := validateDataSourceStrategy(flags, commandDataSourceAnnotation(plan)); err == nil {
		t.Error("plan accepted --data-source local")
	}
	shopper := newCookidooShopperCmd(flags)
	if err := validateDataSourceStrategy(flags, commandDataSourceAnnotation(shopper)); err == nil {
		t.Error("shopper accepted --data-source local")
	}
}

func TestCookidooPlanCompactOutputKeepsIngredientAmounts(t *testing.T) {
	flags := &rootFlags{asJSON: true, compact: true}
	out := cookidooPlannerOutput{Ingredients: []cookidooIngredient{
		{Name: "apple", Quantity: 2, Unit: "pcs"},
		{Name: "butter", Quantity: 10, Unit: "g"},
		{Name: "carrot", Quantity: 3, Unit: "pcs"},
		{Name: "water"},
		{Name: "salt"},
	}}
	var output bytes.Buffer
	if err := printJSONFilteredKeep(&output, out, flags, "quantity", "unit", "details", "raw_lines"); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Ingredients []map[string]any `json:"ingredients"`
	}
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatalf("compact plan output is not JSON: %v", err)
	}
	for _, ingredient := range got.Ingredients[:3] {
		if _, ok := ingredient["quantity"]; !ok {
			t.Fatalf("compact output dropped quantity from %v", ingredient)
		}
		if _, ok := ingredient["unit"]; !ok {
			t.Fatalf("compact output dropped unit from %v", ingredient)
		}
	}
}

func TestParseCookidooRecipeRefRejectsAccountURLsAndBadIDs(t *testing.T) {
	for _, value := range []string{
		"https://cookidoo.thermomix.com/my-week",
		"https://cookidoo.thermomix.com/recipes/recipe/en-US/not-a-recipe",
		"68517",
	} {
		if _, _, _, err := parseCookidooRecipeRef(value, "en-US"); err == nil {
			t.Fatalf("parseCookidooRecipeRef(%q) unexpectedly succeeded", value)
		}
	}
}

func TestParseCookidooIngredient(t *testing.T) {
	got := parseCookidooIngredient("320 g risotto rice")
	if got.Quantity != 320 || got.Unit != "g" || got.Name != "risotto rice" {
		t.Fatalf("unexpected parsed ingredient: %+v", got)
	}
	got = parseCookidooIngredient("Vegetable stock paste (see Tips)")
	if got.Name != "Vegetable stock paste (see Tips)" || got.Quantity != 0 {
		t.Fatalf("unexpected unquantified ingredient: %+v", got)
	}
}
