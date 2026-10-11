// pp:data-source computed
// Hand-authored: persisted default project/org, like neonctl set-context.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type neonContext struct {
	ProjectID string `json:"project_id,omitempty"`
	OrgID     string `json:"org_id,omitempty"`
}

// neonContextPath honors NEON_CONTEXT_FILE, else <user config dir>/neon-pp-cli/context.json.
func neonContextPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("NEON_CONTEXT_FILE")); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "neon-pp-cli", "context.json"), nil
}

func loadNeonContext() neonContext {
	var ctx neonContext
	path, err := neonContextPath()
	if err != nil {
		return ctx
	}
	if raw, err := os.ReadFile(path); err == nil { // #nosec G304 -- path is the user config dir or NEON_CONTEXT_FILE chosen by the user
		_ = json.Unmarshal(raw, &ctx)
	}
	return ctx
}

// defaultNeonProject returns the explicit value, else the saved context project.
func defaultNeonProject(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return loadNeonContext().ProjectID
}

func newNeonContextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "context",
		Short:       "Save a default project and org so commands can omit --project-id.",
		Long:        "Save a default project and org (like 'neonctl set-context'). connection-string, preview up and wait use the saved project when --project-id is omitted. Set NEON_CONTEXT_FILE to use a per-repo context file.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newNeonContextSetCmd(flags))
	cmd.AddCommand(newNeonContextShowCmd(flags))
	cmd.AddCommand(newNeonContextClearCmd(flags))
	return cmd
}

func newNeonContextShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Short:       "Show the saved default project and org.",
		Example:     "  neon-pp-cli context show --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "context show")
			}
			path, err := neonContextPath()
			if err != nil {
				return err
			}
			ctx := loadNeonContext()
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"path": path, "project_id": ctx.ProjectID, "org_id": ctx.OrgID}, flags)
			}
			if ctx.ProjectID == "" && ctx.OrgID == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "no context saved (%s)\n", path)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "project: %s\norg: %s\nfile: %s\n", ctx.ProjectID, ctx.OrgID, path)
			return nil
		},
	}
}

func newNeonContextClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "clear",
		Short:       "Remove the saved context file.",
		Example:     "  neon-pp-cli context clear",
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "context clear")
			}
			path, err := neonContextPath()
			if err != nil {
				return err
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"path": path, "cleared": true}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "cleared %s\n", path)
			return nil
		},
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNeonContextCmd(flags))
	})
}
