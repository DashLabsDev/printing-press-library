// Copyright 2026 Thomas McCormick and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(waitlist-pii-stdin-and-here-checkin): guest name/email/phone stay off argv.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// waitlistPIIFlagNames leak guest identity into shell history and `ps`.
// Submit must not take these as the default input path.
var waitlistPIIFlagNames = []string{
	"email-address",
	"first-name",
	"last-name",
	"primary-phone-area-code",
	"primary-phone-number",
	"primary-phone-extension",
}

var waitlistPIIBodyKeySet = map[string]bool{
	"emailaddress":          true,
	"firstname":             true,
	"lastname":              true,
	"primaryphoneareacode":  true,
	"primaryphonenumber":    true,
	"primaryphoneextension": true,
}

// waitlistSubmitBodyKeys is intentionally a closed list. The upstream
// endpoint does not provide a safe extension mechanism, so unknown stdin
// fields must not silently become a POST body during a preview or live join.
var waitlistSubmitBodyKeys = []string{
	"EmailAddress",
	"FirstName",
	"LastName",
	"IsSmoking",
	"PrimaryPhoneAreaCode",
	"PrimaryPhoneExtension",
	"PrimaryPhoneNumber",
	"PrimaryPhoneType",
	"PartySize",
	"WaitMinutes",
	"Platform",
}

var waitlistSubmitBodyKeySet = map[string]bool{
	"EmailAddress":          true,
	"FirstName":             true,
	"LastName":              true,
	"IsSmoking":             true,
	"PrimaryPhoneAreaCode":  true,
	"PrimaryPhoneExtension": true,
	"PrimaryPhoneNumber":    true,
	"PrimaryPhoneType":      true,
	"PartySize":             true,
	"WaitMinutes":           true,
	"Platform":              true,
}

const waitlistPIIRedacted = "<redacted>"

const waitlistPIIArgvErr = "guest first name, last name, email, and phone must not be passed as argv flags (shell history / ps leak); pass stdin JSON or use an interactive prompt"

const waitlistPIIStdinErr = "guest PII is required on stdin JSON (agents) or an interactive prompt; do not pass name, email, or phone as flags"

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

// RedactWaitlistPII deep-copies body and replaces guest identity fields at
// every map depth. It is deliberately case-insensitive because input from
// generic JSON callers can vary field casing; dry-run display and logs must
// never expose guest name/email/phone merely because an unexpected nesting or
// lowercase key bypassed the canonical request shape.
func RedactWaitlistPII(body map[string]any) map[string]any {
	redacted, ok := redactWaitlistPIIValue(body).(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return redacted
}

func redactWaitlistPIIValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, nested := range typed {
			if waitlistPIIBodyKeySet[strings.ToLower(key)] {
				out[key] = waitlistPIIRedacted
				continue
			}
			out[key] = redactWaitlistPIIValue(nested)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, nested := range typed {
			out[i] = redactWaitlistPIIValue(nested)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, nested := range typed {
			if waitlistPIIBodyKeySet[strings.ToLower(key)] {
				out[key] = waitlistPIIRedacted
				continue
			}
			out[key] = nested
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(typed))
		for i, nested := range typed {
			out[i] = RedactWaitlistPII(nested)
		}
		return out
	default:
		return value
	}
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
	if err := validateWaitlistSubmitBodyFields(jsonBody); err != nil {
		return nil, err
	}
	return jsonBody, nil
}

func validateWaitlistSubmitBodyFields(body map[string]any) error {
	var unknown []string
	for key := range body {
		if !waitlistSubmitBodyKeySet[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	quoted := make([]string, len(unknown))
	for i, key := range unknown {
		quoted[i] = fmt.Sprintf("%q", key)
	}
	fieldLabel := "field"
	if len(quoted) > 1 {
		fieldLabel = "fields"
	}
	return usageErr(fmt.Errorf("unknown stdin JSON %s %s; accepted fields: %s", fieldLabel, strings.Join(quoted, ", "), strings.Join(waitlistSubmitBodyKeys, ", ")))
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
