// Copyright 2026 Thomas McCormick and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(waitlist-pii-stdin-and-here-checkin): guest name/email/phone stay off argv.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// waitlistPIIFlagNames are argv flags that leak guest identity into shell
// history and `ps`. Submit must not take these as the default input path.
var waitlistPIIFlagNames = []string{
	"email-address",
	"first-name",
	"last-name",
	"primary-phone-area-code",
	"primary-phone-number",
	"primary-phone-extension",
}

var waitlistPIIBodyKeys = []string{
	"EmailAddress",
	"FirstName",
	"LastName",
	"PrimaryPhoneAreaCode",
	"PrimaryPhoneNumber",
	"PrimaryPhoneExtension",
}

const waitlistPIIRedacted = "<redacted>"

const waitlistPIIArgvErr = "guest first name, last name, email, and phone must not be passed as argv flags (shell history / ps leak); pass stdin JSON or use an interactive prompt"

const waitlistPIIStdinErr = "guest PII is required on stdin JSON (agents) or an interactive prompt; do not pass name, email, or phone as flags"

// rejectWaitlistPIIFlags refuses PII on argv even when the flags still exist
// for help compatibility. The values must never become the submit body.
func rejectWaitlistPIIFlags(cmd *cobra.Command) error {
	if cmd == nil {
		return nil
	}
	for _, name := range waitlistPIIFlagNames {
		if cmd.Flags().Changed(name) {
			return usageErr(fmt.Errorf("%s", waitlistPIIArgvErr))
		}
	}
	return nil
}

// redactWaitlistPII copies body and replaces guest identity fields. Used for
// --dry-run display and logs so a preview cannot leak name/email/phone.
func redactWaitlistPII(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))
	for k, v := range body {
		out[k] = v
	}
	for _, key := range waitlistPIIBodyKeys {
		if _, ok := out[key]; ok {
			out[key] = waitlistPIIRedacted
		}
	}
	return out
}

func waitlistPIIPlaceholderBody() map[string]any {
	return map[string]any{
		"EmailAddress":         waitlistPIIRedacted,
		"FirstName":            waitlistPIIRedacted,
		"LastName":             waitlistPIIRedacted,
		"PrimaryPhoneAreaCode": waitlistPIIRedacted,
		"PrimaryPhoneNumber":   waitlistPIIRedacted,
	}
}

func waitlistHasGuestPII(body map[string]any) bool {
	for _, key := range []string{"EmailAddress", "FirstName", "LastName", "PrimaryPhoneAreaCode", "PrimaryPhoneNumber"} {
		if waitlistNonEmpty(body[key]) {
			return true
		}
	}
	return false
}

func waitlistNonEmpty(v any) bool {
	if v == nil {
		return false
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s != "" && s != "<nil>"
}

func mergeWaitlistJSON(dst map[string]any, src map[string]any) {
	for k, v := range src {
		dst[k] = v
	}
}

// readWaitlistStdinJSON reads a guest/submit object from cmd.InOrStdin().
func readWaitlistStdinJSON(r io.Reader) (map[string]any, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var jsonBody map[string]any
	if err := json.Unmarshal(data, &jsonBody); err != nil {
		return nil, fmt.Errorf("parsing stdin JSON: %w", err)
	}
	return jsonBody, nil
}

func isTerminalReader(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return isTerminal(f)
	}
	return false
}

func waitlistInteractiveAllowed(flags *rootFlags, in io.Reader) bool {
	if flags != nil && (flags.agent || flags.noInput) {
		return false
	}
	return isTerminalReader(in)
}

func promptWaitlistGuestPII(in io.Reader, errOut io.Writer) (map[string]any, error) {
	reader := bufio.NewReader(in)
	ask := func(label string) (string, error) {
		fmt.Fprintf(errOut, "%s: ", label)
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	first, err := ask("First name")
	if err != nil {
		return nil, err
	}
	last, err := ask("Last name")
	if err != nil {
		return nil, err
	}
	email, err := ask("Email")
	if err != nil {
		return nil, err
	}
	area, err := ask("Phone area code (3 digits)")
	if err != nil {
		return nil, err
	}
	number, err := ask("Phone number (xxx-xxxx)")
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"EmailAddress":         email,
		"FirstName":            first,
		"LastName":             last,
		"PrimaryPhoneAreaCode": area,
		"PrimaryPhoneNumber":   number,
	}
	if !waitlistHasGuestPII(body) {
		return nil, usageErr(fmt.Errorf("%s", waitlistPIIStdinErr))
	}
	return body, nil
}

// collectWaitlistGuestPII loads guest identity from stdin JSON or an
// interactive prompt. Argv flags are never the source.
func collectWaitlistGuestPII(cmd *cobra.Command, flags *rootFlags, stdinBody bool) (map[string]any, error) {
	if err := rejectWaitlistPIIFlags(cmd); err != nil {
		return nil, err
	}
	in := io.Reader(os.Stdin)
	errOut := io.Writer(os.Stderr)
	if cmd != nil {
		in = cmd.InOrStdin()
		errOut = cmd.ErrOrStderr()
	}

	// Piped or --stdin JSON is the agent path.
	if stdinBody || !isTerminalReader(in) {
		parsed, err := readWaitlistStdinJSON(in)
		if err != nil {
			return nil, err
		}
		if parsed != nil {
			return parsed, nil
		}
	}
	if flags != nil && flags.dryRun && !stdinBody {
		return waitlistPIIPlaceholderBody(), nil
	}
	if waitlistInteractiveAllowed(flags, in) {
		return promptWaitlistGuestPII(in, errOut)
	}
	if flags != nil && flags.dryRun {
		return waitlistPIIPlaceholderBody(), nil
	}
	return nil, usageErr(fmt.Errorf("%s", waitlistPIIStdinErr))
}

func applyWaitlistNonPIIFlags(cmd *cobra.Command, body map[string]any, isSmoking bool, partySize float64, waitMinutes int, platform string, phoneType int) {
	if cmd.Flags().Changed("is-smoking") {
		body["IsSmoking"] = isSmoking
	}
	if cmd.Flags().Changed("party-size") || partySize != 0 {
		body["PartySize"] = partySize
	}
	if cmd.Flags().Changed("wait-minutes") || waitMinutes != 0 {
		body["WaitMinutes"] = waitMinutes
	}
	if cmd.Flags().Changed("platform") || platform != "" {
		body["Platform"] = platform
	}
	if cmd.Flags().Changed("primary-phone-type") || phoneType != 0 {
		body["PrimaryPhoneType"] = phoneType
	}
}

func requireWaitlistSubmitFields(body map[string]any, dryRun bool) error {
	if dryRun {
		return nil
	}
	for _, key := range []string{"EmailAddress", "FirstName", "LastName", "PrimaryPhoneAreaCode", "PrimaryPhoneNumber"} {
		if !waitlistNonEmpty(body[key]) || fmt.Sprint(body[key]) == waitlistPIIRedacted {
			return usageErr(fmt.Errorf("%s", waitlistPIIStdinErr))
		}
	}
	if !waitlistNonEmpty(body["PartySize"]) {
		return usageErr(fmt.Errorf("required flag %q not set (or include PartySize in stdin JSON)", "party-size"))
	}
	if !waitlistNonEmpty(body["WaitMinutes"]) {
		return usageErr(fmt.Errorf("required flag %q not set (or include WaitMinutes in stdin JSON)", "wait-minutes"))
	}
	return nil
}
