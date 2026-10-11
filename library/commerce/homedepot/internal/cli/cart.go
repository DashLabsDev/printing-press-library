// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCartCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "cart",
		Short:       "Work with cart",
		Example:     "  homedepot-pp-cli cart add 100001123 --store 4706 --dry-run --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelCartAddCmd(flags))
	cmd.AddCommand(newNovelCartUpdateCmd(flags))
	cmd.AddCommand(newCartShowCmd(flags))
	cmd.AddCommand(newCartRemoveCmd(flags))
	return cmd
}
