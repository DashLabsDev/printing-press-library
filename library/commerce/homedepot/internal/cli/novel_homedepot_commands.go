// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/config"
	"github.com/spf13/cobra"
	"path/filepath"
	"strings"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, group := range root.Commands() {
			switch group.Name() {
			case "cart":
				group.Annotations = map[string]string{"pp:parent-group": "true"}
			case "orders":
				group.Annotations = map[string]string{"pp:parent-group": "true"}
			}
		}
		examples := map[string]string{
			"cart show":              "homedepot-pp-cli cart show --json",
			"cart update":            "homedepot-pp-cli cart update sample-line --quantity 2 --dry-run --json",
			"cart remove":            "homedepot-pp-cli cart remove sample-line --dry-run --json",
			"orders list":            "homedepot-pp-cli orders list --limit 10 --json",
			"orders get":             "homedepot-pp-cli orders get sample-order --dry-run --json",
			"auth status":            "homedepot-pp-cli auth status --verify --json",
			"auth import":            "homedepot-pp-cli auth import --dry-run --json",
			"auth login":             "homedepot-pp-cli auth login --chrome --dry-run --json",
			"auth logout":            "homedepot-pp-cli auth logout --dry-run --json",
			"federation-gateway":     "homedepot-pp-cli federation-gateway --body-json '{}' --dry-run --json",
			"oms get-order-details":  "homedepot-pp-cli oms get-order-details example-value --body-json '{}' --dry-run --json",
			"oms list-order-history": "homedepot-pp-cli oms list-order-history example-value --body-json '{}' --dry-run --json",
		}
		for path, example := range examples {
			cmd, _, err := root.Find(strings.Fields(path))
			if err == nil {
				cmd.Example = "  " + example
			}
		}
		for _, path := range []string{"oms get-order-details", "oms list-order-history"} {
			cmd, _, err := root.Find(strings.Fields(path))
			if err == nil {
				cmd.Annotations["pp:happy-args"] = "userId=example-value;--body-json=example-value"
			}
		}
		cmd, _, err := root.Find([]string{"tail"})
		if err == nil {
			cmd.Annotations["pp:happy-args"] = "--json"
		}

	})
}

// Construct the final auth surface directly so runtime discovery and static
// documentation checks inspect the same commands.
func newHomeDepotAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage authentication for Home Depot", Annotations: map[string]string{"pp:parent-group": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newHomeDepotAuthLoginCmd(flags))
	cmd.AddCommand(newNovelAuthStatusCmd(flags))
	cmd.AddCommand(newHomeDepotAuthImportCmd(flags))
	cmd.AddCommand(newHomeDepotAuthLogoutCmd(flags))
	return cmd
}

// pp:data-source local
func newHomeDepotAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "logout", Short: "Forget this CLI's private browser session", Args: cobra.NoArgs, Annotations: map[string]string{"pp:data-source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) || cliutil.IsAnyHarness() {
			return writeDryRun(cmd.OutOrStdout(), flags, "auth logout")
		}
		path, err := sessionPath()
		if err != nil {
			return err
		}
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			return err
		}
		if err = cfg.ClearTokens(); err != nil {
			return err
		}
		dataDir, err := cliutil.DataDir()
		if err != nil {
			return err
		}
		// Remove the canonical cookie inode rather than following a symlink
		// through the generated jar's write-to-clear implementation.
		if err = os.Remove(filepath.Join(dataDir, "cookies.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"session_present": false, "logged_out": true})
	}}
}

func newHomeDepotAuthStatusCmd(flags *rootFlags) *cobra.Command { return newNovelAuthStatusCmd(flags) }

// pp:data-source local
func newHomeDepotAuthImportCmd(flags *rootFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{Use: "import", Short: "Import and verify a signed-in Safari session or cookie export", Args: cobra.NoArgs, Annotations: map[string]string{"pp:data-source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) || cliutil.IsAnyHarness() {
			return writeDryRun(cmd.OutOrStdout(), flags, "auth import")
		}
		var cookies []homeDepotCookie
		var err error
		if file == "" {
			cookies, err = safariHomeDepotCookies(cmd.Context())
		} else {
			cookies, err = readHomeDepotCookies(file, false)
			if err == nil {
				cookies, err = validHomeDepotCookies(cookies)
			}
		}
		if err != nil {
			return err
		}
		if err = verifyHomeDepotSession(cmd.Context(), flags, cookies); err != nil {
			return err
		}
		// Save only after the candidate passed an isolated protected read. Clear
		// stale credential blobs so they cannot reseed an older account into the jar.
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			return err
		}
		if err = cfg.ClearTokens(); err != nil {
			return err
		}
		if err = saveHomeDepotCookies(cookies); err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"imported": true, "cookie_count": len(cookies), "verified": true})
	}}
	cmd.Flags().StringVar(&file, "file", "", "Read a JSON cookie export instead of Safari")
	return cmd
}

// pp:data-source local
func newHomeDepotAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var chrome bool
	var profile, file string
	cmd := &cobra.Command{Use: "login", Short: "Verify and save a Chrome session with its cookie metadata", Annotations: map[string]string{"pp:data-source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) || cliutil.IsAnyHarness() {
			return writeDryRun(cmd.OutOrStdout(), flags, "auth login")
		}
		if !chrome && file == "" {
			return usageErr(fmt.Errorf("choose --chrome or --cookies-file; for Safari use 'auth import'"))
		}
		if chrome && file != "" {
			return usageErr(fmt.Errorf("choose either --chrome or --cookies-file"))
		}
		var cookies []homeDepotCookie
		var err error
		if chrome {
			cookies, err = chromeHomeDepotCookies(cmd, profile)
		} else {
			cookies, err = readHomeDepotCookies(file, false)
			if err == nil {
				cookies, err = validHomeDepotCookies(cookies)
			}
		}
		if err != nil {
			return err
		}
		if err = verifyHomeDepotSession(cmd.Context(), flags, cookies); err != nil {
			return err
		}
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			return err
		}
		if err = cfg.ClearTokens(); err != nil {
			return err
		}
		if err = saveHomeDepotCookies(cookies); err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"imported": true, "verified": true, "cookie_count": len(cookies)})
	}}
	cmd.Flags().BoolVar(&chrome, "chrome", false, "Read a signed-in Chrome profile's cookies")
	cmd.Flags().BoolVar(&chrome, "browser", false, "Read Chrome cookies (alias for --chrome)")
	cmd.Flags().StringVar(&profile, "profile", "", "Select Chrome profile name or channel/profile")
	cmd.Flags().StringVar(&file, "cookies-file", "", "Read a JSON browser cookie export with metadata")
	return cmd
}
