// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored command group.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPreviewCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Work with preview",
		Example: strings.Trim(`
  neon-pp-cli preview up pr-123 --project-id lucky-sun-123456 --expires 7d --agent
`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "pp:typed-exit-codes": "0,2", "pp:parent-group": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPreviewUpCmd(flags))
	return cmd
}
