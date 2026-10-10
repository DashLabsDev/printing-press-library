// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for pasted cookie formats and the names-only diagnosis.
// Every cookie value here is fake and contains the marker FAKEVAL.

package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const leakMarker = "FAKEVAL"

func fakeCookieList() string {
	return "ahoy_visitor=FAKEVAL-visitor; ahoy_visit=FAKEVAL-visit; X-IC-bcx=FAKEVAL-bcx; " +
		"__Host-instacart_sid=FAKEVAL-sid%3D%3D--abc; build_sha=FAKEVAL-build; " +
		"OptanonConsent=isGpcEnabled=0&datestamp=Thu+Jan+01+2026+00%3A00%3A00+GMT-0500+(FAKEVAL)&version=202409.1.0; " +
		"AMCV_0123456789ABCDEF01234567%40AdobeOrg=FAKEVAL-amcv; _instacart_session_id=FAKEVAL-legacy"
}

// chromeMacCurl mimics Chrome 14x on macOS: "Copy as cURL (bash)" with -b.
func chromeMacCurl(cookieFlag string) string {
	return "curl 'https://sameday.costco.com/graphql?operationName=CartTotals&variables=%7B%22cartId%22%3A%22FAKEVAL%22%7D&extensions=%7B%22persistedQuery%22%3A%7B%22version%22%3A1%7D%7D' \\\n" +
		"  -H 'accept: */*' \\\n" +
		"  -H 'accept-language: en-US,en;q=0.9' \\\n" +
		"  -H 'content-type: application/json' \\\n" +
		"  " + cookieFlag + " \\\n" +
		"  -H 'priority: u=1, i' \\\n" +
		"  -H 'referer: https://sameday.costco.com/store/costco/storefront' \\\n" +
		"  -H 'sec-ch-ua: \"Google Chrome\";v=\"141\", \"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"141\"' \\\n" +
		"  -H 'sec-ch-ua-platform: \"macOS\"' \\\n" +
		"  -H 'user-agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36' \\\n" +
		"  -H 'x-client-identifier: web'"
}

func mustExtract(t *testing.T, in, wantFormat string) string {
	t.Helper()
	h, format, err := extractCookieHeader(in)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	if format != wantFormat {
		t.Fatalf("format = %q, want %q", format, wantFormat)
	}
	kept, _, err := selectSessionCookies(h)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if len(kept) != 8 {
		t.Fatalf("expected 8 cookies, got %d", len(kept))
	}
	return h
}

func TestCurlVariants(t *testing.T) {
	cookies := fakeCookieList()
	cases := []struct{ name, in, format string }{
		{"dash-b", chromeMacCurl("-b '" + cookies + "'"), formatCurlB},
		{"long --cookie", chromeMacCurl("--cookie '" + cookies + "'"), formatCurlB},
		{"--cookie=", chromeMacCurl("'--cookie=" + cookies + "'"), formatCurlB},
		{"-H cookie", chromeMacCurl("-H 'cookie: " + cookies + "'"), formatCurlH},
		{"-H Cookie uppercase", chromeMacCurl("-H 'Cookie: " + cookies + "'"), formatCurlH},
		{"-H COOKIE no space", chromeMacCurl("-H 'COOKIE:" + cookies + "'"), formatCurlH},
		{"ansi-c -b", chromeMacCurl("-b $'" + strings.ReplaceAll(cookies, "(", "\\x28") + "'"), formatCurlB},
		{"ansi-c -H with escaped quote", chromeMacCurl("-H $'cookie: " + cookies + "; note=FAKEVAL\\'q'"), formatCurlH},
		{"CRLF", strings.ReplaceAll(chromeMacCurl("-b '"+cookies+"'"), "\n", "\r\n"), formatCurlB},
		{"trailing newline", chromeMacCurl("-b '"+cookies+"'") + "\n\n", formatCurlB},
		{"single line no continuation", strings.ReplaceAll(chromeMacCurl("-b '"+cookies+"'"), " \\\n", ""), formatCurlB},
		{"zsh prompt prefix", "% " + chromeMacCurl("-b '"+cookies+"'"), formatCurlB},
		{"bash prompt prefix", "$ " + chromeMacCurl("-b '"+cookies+"'"), formatCurlB},
		{"double-quoted -b", chromeMacCurl("-b \"" + cookies + "\""), formatCurlB},
		{"both -b and -H", chromeMacCurl("-b '" + cookies + "' -H 'cookie: ahoy_visit=FAKEVAL-dup'"), formatCurlBH},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extra := 0
			if strings.Contains(tc.name, "escaped quote") {
				extra = 1
			}
			h, format, err := extractCookieHeader(tc.in)
			if err != nil || format != tc.format {
				t.Fatalf("format=%q err=%v", format, err)
			}
			kept, _, err := selectSessionCookies(h)
			if err != nil || len(kept) != 8+extra {
				t.Fatalf("kept=%d err=%v", len(kept), err)
			}
			if kept[3].Name != sessionCookieName && !strings.Contains(h, sessionCookieName+"=FAKEVAL-sid") {
				t.Fatalf("session cookie value not preserved")
			}
		})
	}
}

func TestCurlCmdVariant(t *testing.T) {
	cmd := "curl ^\"https://sameday.costco.com/graphql?operationName=Fake^\" ^\n  -H ^\"accept: */*^\" ^\n  -b ^\"" +
		strings.ReplaceAll(strings.ReplaceAll(fakeCookieList(), "%", "^%"), "&", "^&") + "^\" ^\n  -H ^\"x-client-identifier: web^\""
	h := mustExtract(t, cmd, formatCurlCmd)
	if !strings.Contains(h, "%3D%3D--abc") || !strings.Contains(h, "isGpcEnabled=0&datestamp") {
		t.Fatalf("cmd unescape lost characters")
	}
}

func TestVeryLongCurl(t *testing.T) {
	var b strings.Builder
	b.WriteString(fakeCookieList())
	for i := 0; b.Len() < 15000; i++ {
		b.WriteString("; pad_" + strings.Repeat("x", 3) + string(rune('a'+i%26)) + strings.Repeat("y", i%7) + "=FAKEVAL-" + strings.Repeat("z", 200))
	}
	in := chromeMacCurl("-b '" + b.String() + "'")
	if len(in) < 15000 {
		t.Fatalf("input only %d bytes", len(in))
	}
	h, _, err := extractCookieHeader(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := selectSessionCookies(h); err != nil {
		t.Fatal(err)
	}
	// 200 KB single line also must pass (no scanner token limit).
	huge := chromeMacCurl("-b '" + fakeCookieList() + "; big=FAKEVAL" + strings.Repeat("q", 200000) + "'")
	data, err := readCookieInput(strings.NewReader(huge))
	if err != nil || len(data) != len(huge) {
		t.Fatalf("readCookieInput truncated: %d vs %d err=%v", len(data), len(huge), err)
	}
	if d := diagnoseCookieInput(data, "sameday.costco.com"); d.Stage != stageOK || !d.HasSession {
		t.Fatalf("huge input diag: %+v", d)
	}
}

func stageOf(err error) string {
	var ie *cookieImportError
	if errors.As(err, &ie) {
		return ie.Stage
	}
	return ""
}

func TestEmptyAndWrongFormats(t *testing.T) {
	cases := []struct{ name, in, stage, format, msg string }{
		{"empty", "", stageRead, formatEmpty, "empty"},
		{"whitespace", "  \n\r\n ", stageRead, formatEmpty, "empty"},
		{"browser fetch", "fetch(\"https://sameday.costco.com/graphql?operationName=Fake\", {\n  \"headers\": {\n    \"accept\": \"*/*\",\n    \"x-client-identifier\": \"web\"\n  },\n  \"referrer\": \"https://sameday.costco.com/\",\n  \"body\": null,\n  \"method\": \"GET\",\n  \"mode\": \"cors\",\n  \"credentials\": \"include\"\n});", stageExtract, formatFetch, "Copy as cURL"},
		{"curl without cookie", "curl 'https://sameday.costco.com/graphql' \\\n  -H 'accept: */*' \\\n  -H 'x-client-identifier: web'", stageExtract, formatCurlNoCookie, "no cookie"},
		{"curl -b jar file", "curl 'https://sameday.costco.com/graphql' -b cookies.txt", stageExtract, formatCurlB, "cookie-jar file"},
		{"random multi-line", "hello\nworld", stageDetect, formatUnknown, "no cURL"},
		{"random word", "hello", stageDetect, formatUnknown, "not a Cookie header"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, format, err := extractCookieHeader(tc.in)
			if err == nil || stageOf(err) != tc.stage || format != tc.format || !strings.Contains(err.Error(), tc.msg) {
				t.Fatalf("format=%q err=%v", format, err)
			}
			if !strings.Contains(err.Error(), "stage "+tc.stage) {
				t.Fatalf("error must name the stage: %v", err)
			}
		})
	}
}

func TestNodeFetchAndPowerShell(t *testing.T) {
	node := "fetch(\"https://sameday.costco.com/graphql\", {\n  \"headers\": {\n    \"accept\": \"*/*\",\n    \"cookie\": \"" + fakeCookieList() + "\",\n    \"Referer\": \"https://sameday.costco.com/\"\n  },\n  \"body\": null,\n  \"method\": \"GET\"\n});"
	mustExtract(t, node, formatFetchNode)
	ps := "$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession\n" +
		"$session.Cookies.Add((New-Object System.Net.Cookie(\"__Host-instacart_sid\", \"FAKEVAL-sid\", \"/\", \"sameday.costco.com\")))\n" +
		"$session.Cookies.Add((New-Object System.Net.Cookie(\"ahoy_visit\", \"FAKEVAL-visit\", \"/\", \"sameday.costco.com\")))\n" +
		"Invoke-WebRequest -UseBasicParsing -Uri \"https://sameday.costco.com/graphql\" -WebSession $session"
	h, format, err := extractCookieHeader(ps)
	if err != nil || format != formatPowerShell || h != "__Host-instacart_sid=FAKEVAL-sid; ahoy_visit=FAKEVAL-visit" {
		t.Fatalf("powershell: format=%q err=%v", format, err)
	}
}

func TestRequestHeadersWithCRLF(t *testing.T) {
	in := ":authority: sameday.costco.com\r\n:method: GET\r\naccept: */*\r\nCookie: " + fakeCookieList() + "\r\nx-client-identifier: web\r\n"
	mustExtract(t, in, formatReqHeaders)
}

func TestMissingSessionStageError(t *testing.T) {
	_, _, err := selectSessionCookies("ahoy_visit=FAKEVAL-a; X-IC-bcx=FAKEVAL-b")
	if stageOf(err) != stageRequireSession || strings.Contains(err.Error(), leakMarker) {
		t.Fatalf("unexpected: %v", err)
	}
	_, _, err = selectSessionCookies("justtext")
	if stageOf(err) != stageParsePairs {
		t.Fatalf("expected parse stage, got %v", err)
	}
}

func runAuthWithStdin(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	flags := &rootFlags{noCache: true}
	root := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(newAuthCmd(flags))
	root.SetArgs(append([]string{"auth", "login"}, args...))
	root.SetIn(strings.NewReader(stdin))
	var buf strings.Builder
	root.SetOut(&buf)
	root.SetErr(&buf)
	err := root.Execute()
	return buf.String(), err
}

func TestDiagnoseNeverLeaksOrSaves(t *testing.T) {
	inputs := map[string]string{
		"curl -b":         chromeMacCurl("-b '" + fakeCookieList() + "; WC_USERACTIVITY_12345678=FAKEVAL-id'"),
		"curl no session": chromeMacCurl("-b 'ahoy_visit=FAKEVAL-a; X-IC-bcx=FAKEVAL-b'"),
		"fetch":           "fetch(\"https://sameday.costco.com/graphql\", {\"headers\": {\"accept\": \"FAKEVAL\"}, \"credentials\": \"include\"});",
		"header":          fakeCookieList(),
		"garbage":         "FAKEVAL FAKEVAL",
		"empty":           "",
	}
	for name, in := range inputs {
		t.Run(name, func(t *testing.T) {
			isolatedAuthEnv(t)
			out, err := runAuthWithStdin(t, in, "--cookies-file", "-", "--diagnose")
			all := out
			if err != nil {
				all += err.Error()
			}
			if strings.Contains(all, leakMarker) {
				t.Fatalf("diagnose leaked a value:\n%s", all)
			}
			if strings.Contains(all, "12345678") || strings.Contains(all, "0123456789ABCDEF01234567") {
				t.Fatalf("diagnose leaked an id-like run in a cookie name:\n%s", all)
			}
			for _, want := range []string{"input_bytes:", "format:", "stage_reached:", "cookie_names:", sessionCookieName + ":"} {
				if !strings.Contains(out, want) {
					t.Fatalf("missing %q in:\n%s", want, out)
				}
			}
			status, serr := runCLI(t, []string{"auth", "status"}, authCmds)
			if serr == nil || !strings.Contains(status, "Not authenticated") {
				t.Fatalf("--diagnose must not save credentials:\n%s", status)
			}
		})
	}
}

func TestDiagnoseReportsPresence(t *testing.T) {
	isolatedAuthEnv(t)
	out, err := runAuthWithStdin(t, chromeMacCurl("-b '"+fakeCookieList()+"'"), "--cookies-file", "-", "--diagnose")
	if err != nil {
		t.Fatalf("diagnose failed: %v\n%s", err, out)
	}
	for _, want := range []string{"format:         cURL -b", "stage_reached:  ok", "__Host-instacart_sid: present", "cookie_names:   8", "AMCV_<id>%40AdobeOrg"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	out, err = runAuthWithStdin(t, chromeMacCurl("-b 'ahoy_visit=FAKEVAL-a'"), "--cookies-file", "-", "--diagnose")
	if err == nil || !strings.Contains(out, "__Host-instacart_sid: absent") || !strings.Contains(out, "FAIL at require-session-cookie") {
		t.Fatalf("expected absent + stage failure, err=%v\n%s", err, out)
	}
	out, err = runAuthWithStdin(t, "", "--cookies-file", "-", "--diagnose")
	if err == nil || !strings.Contains(out, "input_bytes:    0") || !strings.Contains(out, "format:         empty") {
		t.Fatalf("expected empty diagnosis, err=%v\n%s", err, out)
	}
}

func TestLoginErrorNamesStage(t *testing.T) {
	isolatedAuthEnv(t)
	_, err := runAuthWithStdin(t, "", "--cookies-file", "-")
	if err == nil || !strings.Contains(err.Error(), "stage read-input") {
		t.Fatalf("empty stdin should fail at read-input: %v", err)
	}
	_, err = runAuthWithStdin(t, chromeMacCurl("-H 'accept: x'"), "--cookies-file", "-")
	if err == nil || !strings.Contains(err.Error(), "stage extract-cookie") || !strings.Contains(err.Error(), "cURL without cookie") {
		t.Fatalf("cookie-less cURL should fail at extract-cookie: %v", err)
	}
	out, err := runAuthWithStdin(t, chromeMacCurl("-b 'ahoy_visit=FAKEVAL-a'"), "--cookies-file", "-")
	if err == nil || !strings.Contains(err.Error(), "stage require-session-cookie") || strings.Contains(out+err.Error(), leakMarker) {
		t.Fatalf("missing session should fail at require-session-cookie without leaking: %v\n%s", err, out)
	}
	out, err = runAuthWithStdin(t, chromeMacCurl("-b '"+fakeCookieList()+"'"), "--cookies-file", "-")
	if err != nil || !strings.Contains(out, "input format: cURL -b") || strings.Contains(out, leakMarker) {
		t.Fatalf("login from Chrome cURL failed: %v\n%s", err, out)
	}
}
