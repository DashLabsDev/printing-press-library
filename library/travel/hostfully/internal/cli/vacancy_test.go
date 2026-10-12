// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/cliutil/testenv"
)

// TestNovelVacancyHelpWires smoke-tests that the vacancy command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelVacancyHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"vacancy", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("vacancy --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "vacancy"} {
		if !strings.Contains(help, want) {
			t.Fatalf("vacancy --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestReviewUnitsUseRegistry(t *testing.T) {
	setUnitRegistry(map[int]string{8: "placeholder-eight", 12: "placeholder-twelve"}, "test")
	t.Cleanup(func() { setUnitRegistry(testUnitUIDs, "test") })
	for _, input := range []string{"12,8,12", "all"} {
		got, err := parseUnits(input)
		if err != nil || !reflect.DeepEqual(got, []int{8, 12}) {
			t.Fatalf("%s: %v, %v", input, got, err)
		}
	}
	for _, input := range []string{"1", "0", "-1", "x"} {
		if _, err := parseUnits(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}
