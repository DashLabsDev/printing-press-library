// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0.
// Hand-authored extension kept outside generated command files.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookidoo/internal/client"
	"github.com/spf13/cobra"
)

// pp:data-source live
// The Cookidoo views deliberately fetch public pages only. They never read or
// write credentials, cookies, account state, My Week, bookmarks, or shopping lists.

type cookidooRecipe struct {
	ID          string         `json:"id"`
	URL         string         `json:"url"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	PrepTime    string         `json:"prepTime,omitempty"`
	CookTime    string         `json:"cookTime,omitempty"`
	TotalTime   string         `json:"totalTime,omitempty"`
	Yield       string         `json:"recipeYield,omitempty"`
	Category    any            `json:"recipeCategory,omitempty"`
	Ingredients []string       `json:"recipeIngredient,omitempty"`
	Nutrition   map[string]any `json:"nutrition,omitempty"`
}

type cookidooIngredient struct {
	Name     string   `json:"name"`
	Details  string   `json:"details,omitempty"`
	Quantity float64  `json:"quantity,omitempty"`
	Unit     string   `json:"unit,omitempty"`
	RawLines []string `json:"raw_lines"`
}

type cookidooPlannerOutput struct {
	Version     string               `json:"version"`
	Locale      string               `json:"locale"`
	Recipes     []cookidooRecipeRef  `json:"recipes"`
	Ingredients []cookidooIngredient `json:"ingredients"`
}

type cookidooRecipeRef struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

const cookidooPlanVersion = "0" + "." + "1"

var cookidooRecipeIDPattern = regexp.MustCompile(`(?i)(r[0-9]+)$`)
var shopperIngredientQuantityPattern = regexp.MustCompile(`^[0-9]+(?:[.,][0-9]+)?(?:\s*-\s*[0-9]+(?:[.,][0-9]+)?)?\s+`)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		recipes := findSubcommand(root, "recipes")
		if recipes == nil {
			return
		}
		addNovelCommandIfAbsent(recipes, newCookidooViewCmd(flags, "details", "Show a compact recipe detail view", func(cmd *cobra.Command, f *rootFlags, c *client.Client, recipes []string, locale string) error {
			if len(recipes) != 1 {
				return fmt.Errorf("details requires exactly one recipe ID or URL")
			}
			recipe, err := fetchCookidooRecipe(cmd, f, c, recipes[0], locale)
			if err != nil {
				return err
			}
			return f.printJSON(cmd, recipe)
		}))
		addNovelCommandIfAbsent(recipes, newCookidooViewCmd(flags, "ingredients", "Show recipe ingredients", func(cmd *cobra.Command, f *rootFlags, c *client.Client, recipes []string, locale string) error {
			if len(recipes) != 1 {
				return fmt.Errorf("ingredients requires exactly one recipe ID or URL")
			}
			recipe, err := fetchCookidooRecipe(cmd, f, c, recipes[0], locale)
			if err != nil {
				return err
			}
			return f.printJSON(cmd, map[string]any{"recipe": recipe.Name, "recipe_id": recipe.ID, "ingredients": recipe.Ingredients})
		}))
		addNovelCommandIfAbsent(recipes, newCookidooViewCmd(flags, "nutrition", "Show recipe nutrition", func(cmd *cobra.Command, f *rootFlags, c *client.Client, recipes []string, locale string) error {
			if len(recipes) != 1 {
				return fmt.Errorf("nutrition requires exactly one recipe ID or URL")
			}
			recipe, err := fetchCookidooRecipe(cmd, f, c, recipes[0], locale)
			if err != nil {
				return err
			}
			return f.printJSON(cmd, map[string]any{"recipe": recipe.Name, "recipe_id": recipe.ID, "nutrition": recipe.Nutrition})
		}))
		addNovelCommandIfAbsent(recipes, newCookidooPlanCmd(flags))
		addNovelCommandIfAbsent(recipes, newCookidooShopperCmd(flags))
	})
}

type cookidooViewRunner func(*cobra.Command, *rootFlags, *client.Client, []string, string) error

func newCookidooViewCmd(flags *rootFlags, name, short string, run cookidooViewRunner) *cobra.Command {
	var values []string
	var locale string
	cmd := &cobra.Command{
		Use:         name + " [recipe-id-or-url]",
		Short:       short,
		Example:     "  cookidoo-pp-cli recipes " + name + " r68517 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			values = append(values, args...)
			if len(values) == 0 {
				return fmt.Errorf("provide a Cookidoo recipe ID or URL")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			return run(cmd, flags, c, values, locale)
		},
	}
	cmd.Flags().StringSliceVar(&values, "recipe", nil, "Recipe ID or public Cookidoo URL (repeatable)")
	cmd.Flags().StringVar(&locale, "locale", "en-US", "Cookidoo locale for bare recipe IDs")
	return cmd
}

func newCookidooPlanCmd(flags *rootFlags) *cobra.Command {
	var values []string
	var locale string
	cmd := &cobra.Command{
		Use:         "plan [recipe-id-or-url ...]",
		Short:       "Aggregate ingredients for recipes into Shopper-ready JSON",
		Example:     "  cookidoo-pp-cli recipes plan r68517 r68522 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			values = append(values, args...)
			if len(values) == 0 {
				return fmt.Errorf("provide at least one Cookidoo recipe ID or URL")
			}
			if locale == "" {
				locale = "en-US"
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out := cookidooPlannerOutput{Version: cookidooPlanVersion, Locale: locale, Recipes: []cookidooRecipeRef{}, Ingredients: []cookidooIngredient{}}
			groups := map[string]*cookidooIngredient{}
			for _, value := range values {
				recipe, err := fetchCookidooRecipe(cmd, flags, c, value, locale)
				if err != nil {
					return err
				}
				out.Recipes = append(out.Recipes, cookidooRecipeRef{ID: recipe.ID, URL: recipe.URL, Name: recipe.Name})
				for _, line := range recipe.Ingredients {
					item := parseCookidooIngredient(line)
					key := strings.ToLower(strings.TrimSpace(item.Name + "|" + item.Unit + "|" + item.Details))
					if existing := groups[key]; existing != nil {
						existing.Quantity += item.Quantity
						existing.RawLines = append(existing.RawLines, line)
					} else {
						item.RawLines = []string{line}
						groups[key] = &item
					}
				}
			}
			for _, item := range groups {
				out.Ingredients = append(out.Ingredients, *item)
			}
			sort.Slice(out.Ingredients, func(i, j int) bool {
				return strings.ToLower(out.Ingredients[i].Name) < strings.ToLower(out.Ingredients[j].Name)
			})
			return printJSONFilteredKeep(cmd.OutOrStdout(), out, flags, "quantity", "unit", "details", "raw_lines")
		},
	}
	cmd.Flags().StringSliceVar(&values, "recipe", nil, "Recipe ID or public Cookidoo URL (repeatable)")
	cmd.Flags().StringVar(&locale, "locale", "en-US", "Default locale for bare recipe IDs")
	return cmd
}

type shopperIngredientMatch struct {
	Ingredient     string          `json:"ingredient"`
	Query          string          `json:"query"`
	Status         string          `json:"status"`
	CandidateCount int             `json:"candidate_count,omitempty"`
	Results        json.RawMessage `json:"results,omitempty"`
	Error          string          `json:"error,omitempty"`
}

type cookidooShopperOutput struct {
	Version string                   `json:"version"`
	Recipe  cookidooRecipeRef        `json:"recipe"`
	Matches []shopperIngredientMatch `json:"matches"`
}

// pp:data-source auto
func newCookidooShopperCmd(flags *rootFlags) *cobra.Command {
	var value string
	var locale string
	var shopperBin string
	var store string
	cmd := &cobra.Command{
		Use:         "shopper <recipe-id-or-url>",
		Short:       "Find recipe ingredients in Shopper's product catalog",
		Example:     "  cookidoo-pp-cli recipes shopper r68517 --shopper-bin shopper-pp-cli --json",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:blocked-flags": "shopper-bin", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if value == "" && len(args) == 1 {
				value = args[0]
			}
			if value == "" {
				return fmt.Errorf("provide a Cookidoo recipe ID or URL")
			}
			if shopperBin == "" {
				shopperBin = "shopper-pp-cli"
			}
			if _, err := exec.LookPath(shopperBin); err != nil {
				return fmt.Errorf("Shopper CLI not found at %q; install it or pass --shopper-bin: %w", shopperBin, err)
			}
			cookidooClient, err := flags.newClient()
			if err != nil {
				return err
			}
			recipe, err := fetchCookidooRecipe(cmd, flags, cookidooClient, value, locale)
			if err != nil {
				return err
			}
			out := cookidooShopperOutput{Version: cookidooPlanVersion, Recipe: cookidooRecipeRef{ID: recipe.ID, URL: recipe.URL, Name: recipe.Name}, Matches: []shopperIngredientMatch{}}
			for _, line := range recipe.Ingredients {
				query := shopperIngredientQuery(line)
				if query == "" {
					continue
				}
				match := shopperIngredientMatch{Ingredient: line, Query: query}
				args := []string{"catalog", "create-search", "--query", query, "--agent", "--data-source", "auto", "--no-learn"}
				if store != "" {
					args = append(args, "--store", store)
				}
				if flags.noCache {
					args = append(args, "--no-cache")
				}
				var stdout, stderr bytes.Buffer
				process := exec.CommandContext(cmd.Context(), shopperBin, args...)
				process.Stdout, process.Stderr = &stdout, &stderr
				if err := process.Run(); err != nil {
					match.Status = "error"
					match.Error = strings.TrimSpace(stderr.String())
					if match.Error == "" {
						match.Error = err.Error()
					}
				} else {
					match.Results = json.RawMessage(bytes.TrimSpace(stdout.Bytes()))
					match.CandidateCount = shopperCandidateCount(match.Results)
					if match.CandidateCount > 0 {
						match.Status = "candidates"
					} else {
						match.Status = "no_match"
					}
				}
				out.Matches = append(out.Matches, match)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&value, "recipe", "", "Recipe ID or public Cookidoo URL")
	cmd.Flags().StringVar(&locale, "locale", "en-US", "Cookidoo locale for bare recipe IDs")
	cmd.Flags().StringVar(&shopperBin, "shopper-bin", "shopper-pp-cli", "Shopper CLI executable")
	cmd.Flags().StringVar(&store, "store", "", "Shopper store override, for example programada or unica")
	return cmd
}

func shopperIngredientQuery(line string) string {
	query := strings.TrimSpace(shopperIngredientQuantityPattern.ReplaceAllString(line, ""))
	fields := strings.Fields(query)
	if len(fields) > 1 && cookidooIngredientUnits[strings.ToLower(fields[0])] {
		query = strings.TrimSpace(strings.Join(fields[1:], " "))
	}
	return query
}

func shopperCandidateCount(raw json.RawMessage) int {
	var envelope struct {
		Results struct {
			Products []json.RawMessage `json:"products"`
		} `json:"results"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return 0
	}
	return len(envelope.Results.Products)
}

func fetchCookidooRecipe(cmd *cobra.Command, flags *rootFlags, c *client.Client, value, defaultLocale string) (cookidooRecipe, error) {
	locale, id, pageURL, err := parseCookidooRecipeRef(value, defaultLocale)
	if err != nil {
		return cookidooRecipe{}, err
	}
	path := "/recipes/recipe/" + url.PathEscape(locale) + "/" + url.PathEscape(id)
	data, err := c.GetWithHeaders(cmd.Context(), path, nil, map[string]string{
		"Accept":                  "text/html,application/xhtml+xml",
		client.HTMLResponseHeader: "true",
	})
	if err != nil {
		return cookidooRecipe{}, classifyAPIError(cmd.ErrOrStderr(), err, flags)
	}
	if c.IsDryRun() {
		return cookidooRecipe{ID: id, URL: pageURL, Name: "dry-run"}, nil
	}
	data, err = ExtractCookidooRecipeHTML(data, c.RequestBaseURL()+path, c.LastContentType())
	if err != nil {
		return cookidooRecipe{}, err
	}
	var recipe cookidooRecipe
	if err := json.Unmarshal(data, &recipe); err != nil {
		return cookidooRecipe{}, err
	}
	recipe.ID, recipe.URL = id, pageURL
	return recipe, nil
}

func parseCookidooRecipeRef(value, defaultLocale string) (string, string, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", "", errors.New("recipe ID or URL cannot be empty")
	}
	if strings.HasPrefix(strings.ToLower(value), "http://") || strings.HasPrefix(strings.ToLower(value), "https://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid recipe URL: %w", err)
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 4 || parts[len(parts)-4] != "recipes" || parts[len(parts)-3] != "recipe" {
			return "", "", "", fmt.Errorf("unsupported Cookidoo recipe URL: %s", value)
		}
		id := parts[len(parts)-1]
		if !cookidooRecipeIDPattern.MatchString(id) {
			return "", "", "", fmt.Errorf("recipe URL has no recipe ID: %s", value)
		}
		return parts[len(parts)-2], id, value, nil
	}
	if !cookidooRecipeIDPattern.MatchString(value) {
		return "", "", "", fmt.Errorf("recipe ID must look like r68517 or be a public Cookidoo recipe URL")
	}
	return defaultLocale, value, "https://cookidoo.thermomix.com/recipes/recipe/" + url.PathEscape(defaultLocale) + "/" + url.PathEscape(value), nil
}

func parseCookidooIngredient(line string) cookidooIngredient {
	fields := strings.Fields(line)
	item := cookidooIngredient{Name: line}
	if len(fields) >= 2 {
		if quantity, err := strconv.ParseFloat(strings.ReplaceAll(fields[0], ",", "."), 64); err == nil {
			item.Quantity = quantity
			if cookidooIngredientUnits[strings.ToLower(fields[1])] {
				item.Unit, item.Name = fields[1], strings.TrimSpace(strings.Join(fields[2:], " "))
			} else {
				item.Name = strings.TrimSpace(strings.Join(fields[1:], " "))
			}
		}
	}
	return item
}

var cookidooIngredientUnits = map[string]bool{
	"g": true, "kg": true, "mg": true, "ml": true, "l": true,
	"tsp": true, "tbsp": true, "teaspoon": true, "teaspoons": true,
	"tablespoon": true, "tablespoons": true, "cup": true, "cups": true,
	"pinch": true, "clove": true, "cloves": true, "piece": true, "pieces": true,
}
