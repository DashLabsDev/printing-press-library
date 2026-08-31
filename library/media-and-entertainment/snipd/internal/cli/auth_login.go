// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command — NOT generator output.
// pp:data-source live
//
// auth login turns Snipd's UUID device-pairing flow into one command. Without
// it the user has to generate a UUID by hand, sign in, then open a raw JSON
// endpoint and copy a secret out of the response body — the step no
// non-technical user discovers unaided. Same shape as `gh auth login`: open the
// browser, wait, save.
//
// The pairing mechanics live in internal/snipd/auth.go; this file is wiring,
// side-effect discipline, and output.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/snipd"
	"github.com/spf13/cobra"
)

// loginTimeout bounds the whole pairing wait. The server long-holds each poll,
// so this is a human deadline (find the tab, pick a provider, sign in), not a
// network one. The root --timeout flag overrides it when set larger.
const loginTimeout = 3 * time.Minute

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var noBrowser bool
	var pairingUUID string

	cmd := &cobra.Command{
		Use: "login",
		// "connect" is the word on the Snipd plugin's own button — cheap to
		// honour for anyone arriving from those docs.
		Aliases: []string{"connect"},
		Short:   "Sign in to Snipd in your browser and save the token automatically",
		Example: "  snipd-pp-cli auth login\n  snipd-pp-cli auth login --no-browser",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			if pairingUUID == "" {
				pairingUUID = uuid.NewString()
			} else if _, err := uuid.Parse(pairingUUID); err != nil {
				return usageErr(fmt.Errorf("--uuid must be a valid UUID: %w", err))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			signIn := snipd.SignInURL(pairingUUID)

			// Side-effect discipline. --dry-run and the verifier's mock mode
			// must never pop a browser tab. Live dogfood is excluded too, for a
			// different reason: pairing cannot complete without a human, so a
			// real attempt would hang until the runner's flat per-command
			// timeout and report a false failure.
			if dryRunOK(flags) || cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				return writeLoginDryRun(w, flags, signIn)
			}

			// --no-input means nobody is at the keyboard to dismiss a browser
			// window; print the URL and still poll, so a paired sign-in from
			// another device works.
			launch := !noBrowser && !flags.noInput

			fmt.Fprintln(w, "Sign in to Snipd to authorise this CLI:")
			fmt.Fprintln(w, "  "+signIn)
			if launch {
				if err := openInBrowser(signIn); err != nil {
					fmt.Fprintf(w, "  (couldn't open a browser automatically: %v — open the URL above)\n", err)
				}
			}
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Waiting for the sign-in to complete...")

			timeout := loginTimeout
			if flags.timeout > timeout {
				timeout = flags.timeout
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			token, err := snipd.PollForToken(ctx, cfg.BaseURL, pairingUUID)
			if err != nil {
				if ctx.Err() != nil || err == snipd.ErrSignInIncomplete {
					return authErr(fmt.Errorf("no token received — the sign-in did not finish in time.\n" +
						"Run 'snipd-pp-cli auth login' again, or use 'snipd-pp-cli auth setup' for the manual steps"))
				}
				return authErr(err)
			}

			// Clear any legacy auth_header first, for the same reason
			// set-token does: a pre-existing value shadows the saved
			// credential via AuthHeader() and the login silently has no effect.
			cfg.AuthHeaderVal = ""
			if err := cfg.SaveTokens("", "", token, "", cfg.TokenExpiry); err != nil {
				return configErr(fmt.Errorf("saving token: %w", err))
			}

			savePath := credentialSavePath(cfg)
			if flags.asJSON {
				out := map[string]any{
					"authenticated": true,
					"saved":         true,
					"token":         snipd.MaskToken(token),
					"config_path":   cfg.Path,
				}
				if !cfg.AgentcookieManagedByExternalStore() {
					out["credentials_path"] = savePath
				}
				return printJSONFiltered(w, out, flags)
			}

			fmt.Fprintln(w, green("Signed in to Snipd."))
			fmt.Fprintf(w, "  Token: %s\n", snipd.MaskToken(token))
			fmt.Fprintf(w, "  Saved to: %s\n", savePath)
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Next: snipd-pp-cli pull   (builds your local snip mirror)")
			return nil
		},
	}

	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the sign-in URL instead of opening a browser (headless/SSH)")
	cmd.Flags().StringVar(&pairingUUID, "uuid", "", "Use a fixed pairing UUID instead of generating one")
	_ = cmd.Flags().MarkHidden("uuid")

	return cmd
}

// openInBrowser hands a URL to the OS handler. Only ever called with a URL this
// process built from a validated UUID — never with user-supplied text.
func openInBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

// writeLoginDryRun reports the suppressed side effect in whichever format the
// caller asked for. The sign-in URL is included because it is the one piece a
// human running --dry-run actually wants, and it carries no secret — the
// pairing UUID is a random throwaway correlator.
func writeLoginDryRun(w io.Writer, flags *rootFlags, signIn string) error {
	const action = "login"
	would := "run " + action + "; no changes made"
	if flags != nil && flags.asJSON {
		return json.NewEncoder(w).Encode(dryRunResult{
			DryRun: true, Action: action, Would: would, URL: signIn,
		})
	}
	if _, err := fmt.Fprintf(w, "dry-run: would %s\n", would); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "would open:", signIn)
	return err
}
