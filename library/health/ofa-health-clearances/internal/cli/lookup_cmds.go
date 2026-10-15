// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/internal/ofa"
)

func newBreedsCmd(flags *rootFlags) *cobra.Command {
	var species string
	cmd := &cobra.Command{
		Use:   "breeds [query]",
		Short: "Look up OFA breed codes by code or name (offline, embedded table)",
		Long: `Look up the breed codes OFA's search uses. Matches an exact code or any
part of the breed name. Pass the code to --breed on 'results' and 'kennels'.`,
		Example:     "  ofa-health-clearances-pp-cli breeds koolie\n  ofa-health-clearances-pp-cli breeds shepherd --species dog --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if species != "dog" && species != "cat" && species != "all" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--species must be dog, cat, or all"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search the embedded OFA breed table")
			}
			found := ofa.FindBreeds(strings.Join(args, " "), species)
			if len(found) == 0 && len(args) > 0 {
				return notFoundErr(fmt.Errorf("no breed matches %q", strings.Join(args, " ")))
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAutoTable(cmd.OutOrStdout(), toMaps(found))
			}
			return printJSONFiltered(cmd.OutOrStdout(), found, flags)
		},
	}
	cmd.Flags().StringVar(&species, "species", "dog", "dog, cat, or all")
	return cmd
}

func newTestsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tests [query]",
		Short: "Look up OFA health registry (test) codes by code, name, method, or category (offline)",
		Long: `Look up the registry codes OFA's search uses, such as HD (hips), EL (elbows),
or EYE (eyes). Matches the code, or any part of the name, method (Radiological,
Lab Test, Physical Exam), or category. Pass the code to --test.`,
		Example:     "  ofa-health-clearances-pp-cli tests hips\n  ofa-health-clearances-pp-cli tests cardiac --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search the embedded OFA test table")
			}
			found := ofa.FindTests(strings.Join(args, " "))
			if len(found) == 0 && len(args) > 0 {
				return notFoundErr(fmt.Errorf("no OFA test matches %q", strings.Join(args, " ")))
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAutoTable(cmd.OutOrStdout(), toMaps(found))
			}
			return printJSONFiltered(cmd.OutOrStdout(), found, flags)
		},
	}
	return cmd
}
