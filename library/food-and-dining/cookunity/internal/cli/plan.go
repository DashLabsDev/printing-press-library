// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: offline macro-constrained meal planner.
// Selects a set of meals from the local mirror that collectively fit
// protein / calorie / budget / diet constraints. Preserved on regen.
package cli

import (
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/types"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelPlanCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		proteinMin  float64
		caloriesMax int
		count       int
		budget      float64
		diet        string
		cuisine     string
		excludeAllg string
	)

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Auto-build a set of meals that collectively hit your protein, calorie, budget, and diet targets — offline",
		Long: `Build a meal set from the locally-synced menu that fits your constraints.

Selection is greedy: eligible meals (matching --diet/--cuisine, under
--calories-max per meal, excluding --exclude-allergen) are ranked by protein
per dollar, then chosen highest-first while total price stays within --budget,
until --count meals are selected.

Run 'cookunity-pp-cli sync' first to populate the local store.`,
		Example:     "  cookunity-pp-cli plan --protein-min 40 --calories-max 600 --count 8 --diet gluten-free --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			path := resolveMealDBPath(dbPath)
			if !mealDBExists(path) {
				return missingMirror(cmd, path, flags)
			}
			meals, err := loadMeals(ctx, path)
			if err != nil {
				return err
			}
			// Eligibility filter (per-meal constraints).
			eligible := applyMealFilter(meals, mealFilter{
				diet: diet, cuisine: cuisine, maxCalories: caloriesMax,
				excludeAllergen: excludeAllg, inStockOnly: true,
			})
			// Rank by protein-per-dollar, highest first.
			sort.SliceStable(eligible, func(i, j int) bool {
				return proteinPerDollar(eligible[i]) > proteinPerDollar(eligible[j])
			})

			var (
				selected  []types.Meal
				totalCals int
				totalProt float64
				totalCost float64
			)
			for _, m := range eligible {
				if count > 0 && len(selected) >= count {
					break
				}
				if budget > 0 && totalCost+m.FinalPrice > budget {
					continue
				}
				selected = append(selected, m)
				totalCals += m.Calories
				totalProt += m.Protein
				totalCost += m.FinalPrice
			}

			result := map[string]any{
				"meals":            selected,
				"count":            len(selected),
				"total_calories":   totalCals,
				"total_protein":    round1(totalProt),
				"total_cost":       round2(totalCost),
				"protein_target":   proteinMin,
				"protein_achieved": round1(totalProt),
				"meets_protein":    proteinMin == 0 || totalProt >= proteinMin,
			}
			if len(selected) == 0 {
				result["note"] = "no meals matched the constraints; loosen --diet/--calories-max or run 'cookunity-pp-cli sync' for the target week"
			}
			return flags.printJSON(cmd, result)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	cmd.Flags().Float64Var(&proteinMin, "protein-min", 0, "Target minimum total protein grams (reported; selection maximizes protein)")
	cmd.Flags().IntVar(&caloriesMax, "calories-max", 0, "Maximum calories per meal (0 = no cap)")
	cmd.Flags().IntVar(&count, "count", 8, "Number of meals to select")
	cmd.Flags().Float64Var(&budget, "budget", 0, "Maximum total price across selected meals (0 = no cap)")
	cmd.Flags().StringVar(&diet, "diet", "", "Restrict to a diet tag (e.g. gluten-free, high-protein, keto)")
	cmd.Flags().StringVar(&cuisine, "cuisine", "", "Restrict to a cuisine")
	cmd.Flags().StringVar(&excludeAllg, "exclude-allergen", "", "Exclude meals with these allergens (comma-separated)")
	return cmd
}

func proteinPerDollar(m types.Meal) float64 {
	if m.FinalPrice <= 0 {
		return 0
	}
	return m.Protein / m.FinalPrice
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
