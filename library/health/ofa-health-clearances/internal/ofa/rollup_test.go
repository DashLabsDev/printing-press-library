// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package ofa

import (
	"strings"
	"testing"
)

func TestGrade(t *testing.T) {
	cases := []struct{ num, test, want string }{
		{"AUK-50E24M-C-NOPI", "HIPS", "EXCELLENT"},
		{"AS-42855G24F-U-VPI", "HIPS", "GOOD"},
		{"XX-7F30M-PI", "hips", "FAIR"},
		{"AUK-EL50M24-C-NOPI", "ELBOW", ""},
		{"AUK-EL50M24-C-NOPI", "HIPS", ""},
		{"", "HIPS", ""},
	}
	for _, c := range cases {
		if got := Grade(c.num, c.test); got != c.want {
			t.Errorf("Grade(%q,%q)=%q want %q", c.num, c.test, got, c.want)
		}
	}
}

func TestFlag(t *testing.T) {
	cases := map[string]string{"": "no-number", "  ": "no-number", "AS-PRA-CAR123/24F": "carrier", "AS-MDR1-CL12/24M": "", "AUK-51G26F-C-VPI": ""}
	for in, want := range cases {
		if got := Flag(in); got != want {
			t.Errorf("Flag(%q)=%q want %q", in, got, want)
		}
	}
}

func TestKennelPrefix(t *testing.T) {
	cases := []struct {
		name  string
		words int
		want  string
	}{
		{"FAKERIDGE DAISY GIRL", 0, "FAKERIDGE"},
		{"3 OAK MADEUP'S FINE EDGE", 0, "3 OAK"},
		{"GCHB CH FAKEVALE JOHN", 0, "FAKEVALE"},
		{"PRETENDIA'S BLUE", 0, "PRETENDIA"},
		{"BLUE CART ROVER", 2, "BLUE CART"},
		{"SOLO", 3, "SOLO"},
		{"", 0, ""},
	}
	for _, c := range cases {
		if got := KennelPrefix(c.name, c.words); got != c.want {
			t.Errorf("KennelPrefix(%q,%d)=%q want %q", c.name, c.words, got, c.want)
		}
	}
}

func TestRollupRanksAndCounts(t *testing.T) {
	rows := []Row{
		{AppNum: "1", Name: "ALPHA ONE", CHIC: true, Test: "HIPS", TestDate: "2026-01-01", Grade: "GOOD"},
		{AppNum: "1", Name: "ALPHA ONE", CHIC: true, Test: "ELBOW", TestDate: "2026-02-01"},
		{AppNum: "2", Name: "ALPHA TWO", Test: "HIPS", TestDate: "2025-06-01", Grade: "EXCELLENT"},
		{AppNum: "3", Name: "BETA ONE", Test: "HIPS", TestDate: "2026-03-01", Flag: "no-number"},
	}
	ks := Rollup(rows, 0)
	if len(ks) != 2 || ks[0].Prefix != "ALPHA" || ks[1].Prefix != "BETA" {
		t.Fatalf("ranking wrong: %+v", ks)
	}
	a := ks[0]
	if a.Dogs != 2 || a.CHICDogs != 1 || a.Records != 3 || a.LatestTest != "2026-02-01" {
		t.Errorf("alpha counts wrong: %+v", a)
	}
	if strings.Join(a.Tests, ",") != "ELBOW,HIPS" || a.HipGrades["GOOD"] != 1 || a.HipGrades["EXCELLENT"] != 1 {
		t.Errorf("alpha tests/grades wrong: %+v", a)
	}
	if ks[1].Flagged != 1 || ks[1].HipGrades != nil {
		t.Errorf("beta flags/grades wrong: %+v", ks[1])
	}
}

func TestRollupEmpty(t *testing.T) {
	if ks := Rollup(nil, 0); len(ks) != 0 {
		t.Errorf("want no kennels, got %+v", ks)
	}
}

func TestLookups(t *testing.T) {
	if n := len(FindBreeds("", "all")); n < 400 {
		t.Fatalf("embedded breeds = %d, want ~480", n)
	}
	if n := len(FindTests("")); n < 200 {
		t.Fatalf("embedded tests = %d, want ~242", n)
	}
	for in, want := range map[string]string{"AUK": "AUK", "auk": "AUK", "australian koolie": "AUK", "koolie": "AUK"} {
		if got, err := ResolveBreed(in); err != nil || got != want {
			t.Errorf("ResolveBreed(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	if _, err := ResolveBreed("kelpie"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("kelpie should be ambiguous (AKP, WK): %v", err)
	}
	if _, err := ResolveBreed("zzznotabreed"); err == nil {
		t.Error("unknown breed should error")
	}
	for in, want := range map[string]string{"HD": "HD", "hips": "HD", "elbow": "EL"} {
		if got, err := ResolveTest(in); err != nil || got != want {
			t.Errorf("ResolveTest(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	if got := FindBreeds("koolie", "dog"); len(got) != 1 || got[0].Code != "AUK" {
		t.Errorf("FindBreeds(koolie)=%+v", got)
	}
	if got := FindTests("radiological"); len(got) == 0 {
		t.Error("FindTests by method found nothing")
	}
}
