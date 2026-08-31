// Copyright 2026 Thomas McCormick and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(waitlist-pii-off-argv): guest name/email/phone stay off argv by default.

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

// waitlistPIIFlagNames leak guest identity into shell history and ps.
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

const waitlistPIIArgvErr = "guest first name, last name, email, and phone must not be passed as argv flags unless --yes is set (private confirmed path); pass stdin JSON, --guest-file, or TEXAS_ROADHOUSE_GUEST_* env vars"

const waitlistPIIStdinErr = "guest PII is required on stdin JSON, --guest-file, TEXAS_ROADHOUSE_GUEST_* env vars, or an interactive prompt; do not pass name, email, or phone as flags without --yes"

const (
	waitlistGuestEmailEnv     = "TEXAS_ROADHOUSE_GUEST_EMAIL"
	waitlistGuestFirstEnv     = "TEXAS_ROADHOUSE_GUEST_FIRST_NAME"
	waitlistGuestLastEnv      = "TEXAS_ROADHOUSE_GUEST_LAST_NAME"
	waitlistGuestPhoneAreaEnv = "TEXAS_ROADHOUSE_GUEST_PHONE_AREA_CODE"
	waitlistGuestPhoneNumEnv  = "TEXAS_ROADHOUSE_GUEST_PHONE_NUMBER"
)

type waitlistPIIFlagValues struct {
	email, first, last, area, number, ext string
}

func waitlistPIIFlagsChanged(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	for _, name := range waitlistPIIFlagNames {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func rejectWaitlistPIIFlagsUnlessYes(cmd *cobra.Command, flags *rootFlags) error {
	if !waitlistPIIFlagsChanged(cmd) {
		return nil
	}
	if flags != nil && flags.yes {
		return nil
	}
	return usageErr(fmt.Errorf("%s", waitlistPIIArgvErr))
}

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
		if waitlistNonEmpty(body[key]) && fmt.Sprint(body[key]) != waitlistPIIRedacted {
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
		if waitlistNonEmpty(v) {
			dst[k] = v
		}
	}
}

func readWaitlistJSON(r io.Reader) (map[string]any, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading guest JSON: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var jsonBody map[string]any
	if err := json.Unmarshal(data, &jsonBody); err != nil {
		return nil, fmt.Errorf("parsing guest JSON: %w", err)
	}
	return jsonBody, nil
}

func readWaitlistGuestFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading --guest-file: %w", err)
	}
	var jsonBody map[string]any
	if err := json.Unmarshal(data, &jsonBody); err != nil {
		return nil, fmt.Errorf("parsing --guest-file JSON: %w", err)
	}
	return jsonBody, nil
}

func waitlistGuestEnvBody() map[string]any {
	body := map[string]any{}
	if v := strings.TrimSpace(os.Getenv(waitlistGuestEmailEnv)); v != "" {
		body["EmailAddress"] = v
	}
	if v := strings.TrimSpace(os.Getenv(waitlistGuestFirstEnv)); v != "" {
		body["FirstName"] = v
	}
	if v := strings.TrimSpace(os.Getenv(waitlistGuestLastEnv)); v != "" {
		body["LastName"] = v
	}
	if v := strings.TrimSpace(os.Getenv(waitlistGuestPhoneAreaEnv)); v != "" {
		body["PrimaryPhoneAreaCode"] = v
	}
	if v := strings.TrimSpace(os.Getenv(waitlistGuestPhoneNumEnv)); v != "" {
		body["PrimaryPhoneNumber"] = v
	}
	return body
}

func waitlistFlagPIIBody(vals waitlistPIIFlagValues) map[string]any {
	body := map[string]any{}
	if strings.TrimSpace(vals.email) != "" {
		body["EmailAddress"] = vals.email
	}
	if strings.TrimSpace(vals.first) != "" {
		body["FirstName"] = vals.first
	}
	if strings.TrimSpace(vals.last) != "" {
		body["LastName"] = vals.last
	}
	if strings.TrimSpace(vals.area) != "" {
		body["PrimaryPhoneAreaCode"] = vals.area
	}
	if strings.TrimSpace(vals.number) != "" {
		body["PrimaryPhoneNumber"] = vals.number
	}
	if strings.TrimSpace(vals.ext) != "" {
		body["PrimaryPhoneExtension"] = vals.ext
	}
	return body
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

func collectWaitlistGuestPII(cmd *cobra.Command, flags *rootFlags, stdinBody bool, guestFile string, flagVals waitlistPIIFlagValues) (map[string]any, error) {
	if err := rejectWaitlistPIIFlagsUnlessYes(cmd, flags); err != nil {
		return nil, err
	}
	in := io.Reader(os.Stdin)
	errOut := io.Writer(os.Stderr)
	if cmd != nil {
		in = cmd.InOrStdin()
		errOut = cmd.ErrOrStderr()
	}

	body := map[string]any{}
	if strings.TrimSpace(guestFile) != "" {
		parsed, err := readWaitlistGuestFile(guestFile)
		if err != nil {
			return nil, err
		}
		mergeWaitlistJSON(body, parsed)
	}

	if stdinBody || !isTerminalReader(in) {
		parsed, err := readWaitlistJSON(in)
		if err != nil {
			return nil, err
		}
		if parsed != nil {
			mergeWaitlistJSON(body, parsed)
		}
	}

	mergeWaitlistJSON(body, waitlistGuestEnvBody())

	if flags != nil && flags.yes && waitlistPIIFlagsChanged(cmd) {
		mergeWaitlistJSON(body, waitlistFlagPIIBody(flagVals))
	}

	if waitlistHasGuestPII(body) {
		return body, nil
	}
	if flags != nil && flags.dryRun && !stdinBody && strings.TrimSpace(guestFile) == "" {
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
