// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package ofa

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Breed is one OFA breed code from the Advanced Search form.
type Breed struct {
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Species string   `json:"species"`
	Groups  []string `json:"groups,omitempty"`
}

// Test is one OFA health registry (test) code from the Advanced Search form.
type Test struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Method   string `json:"method"`
	Category string `json:"category"`
}

//go:embed data/breeds.json
var breedsJSON []byte

//go:embed data/tests.json
var testsJSON []byte

var (
	loadOnce sync.Once
	breeds   []Breed
	tests    []Test
)

func load() {
	loadOnce.Do(func() {
		if err := json.Unmarshal(breedsJSON, &breeds); err != nil {
			panic(fmt.Sprintf("ofa: embedded breeds.json: %v", err))
		}
		if err := json.Unmarshal(testsJSON, &tests); err != nil {
			panic(fmt.Sprintf("ofa: embedded tests.json: %v", err))
		}
	})
}

// BreedByName returns the breed whose display name equals name, or nil.
func BreedByName(name string) *Breed {
	load()
	for i := range breeds {
		if strings.EqualFold(breeds[i].Name, name) {
			return &breeds[i]
		}
	}
	return nil
}

// FindBreeds returns breeds whose code equals q or whose name contains q
// (case-insensitive). species "" or "all" matches every species.
func FindBreeds(q, species string) []Breed {
	load()
	q = strings.ToUpper(strings.TrimSpace(q))
	var out []Breed
	for _, b := range breeds {
		if species != "" && species != "all" && b.Species != species {
			continue
		}
		if q == "" || b.Code == q || strings.Contains(b.Name, q) {
			out = append(out, b)
		}
	}
	return out
}

// FindTests returns tests whose code equals q or whose name, method, or
// category contains q (case-insensitive).
func FindTests(q string) []Test {
	load()
	q = strings.ToUpper(strings.TrimSpace(q))
	var out []Test
	for _, t := range tests {
		if q == "" || t.Code == q || strings.Contains(t.Name, q) ||
			strings.Contains(strings.ToUpper(t.Method), q) || strings.Contains(strings.ToUpper(t.Category), q) {
			out = append(out, t)
		}
	}
	return out
}

// ResolveBreed maps a breed code or unambiguous name fragment to its code.
func ResolveBreed(input string) (string, error) {
	load()
	in := strings.ToUpper(strings.TrimSpace(input))
	for _, b := range breeds {
		if b.Code == in {
			return b.Code, nil
		}
	}
	if b := BreedByName(in); b != nil {
		return b.Code, nil
	}
	matches := FindBreeds(in, "all")
	switch len(matches) {
	case 1:
		return matches[0].Code, nil
	case 0:
		return "", fmt.Errorf("unknown breed %q; run 'ofa-health-clearances-pp-cli breeds %s' to search codes", input, strings.ToLower(input))
	default:
		return "", fmt.Errorf("breed %q is ambiguous (%s); pass the code", input, codeList(len(matches), func(i int) string { return matches[i].Code + "=" + matches[i].Name }))
	}
}

// ResolveTest maps a test code or unambiguous name fragment to its code.
func ResolveTest(input string) (string, error) {
	load()
	in := strings.ToUpper(strings.TrimSpace(input))
	for _, t := range tests {
		if t.Code == in || t.Name == in {
			return t.Code, nil
		}
	}
	var matches []Test
	for _, t := range tests {
		if strings.Contains(t.Name, in) {
			matches = append(matches, t)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0].Code, nil
	case 0:
		return "", fmt.Errorf("unknown test %q; run 'ofa-health-clearances-pp-cli tests %s' to search codes", input, strings.ToLower(input))
	default:
		return "", fmt.Errorf("test %q is ambiguous (%s); pass the code", input, codeList(len(matches), func(i int) string { return matches[i].Code + "=" + matches[i].Name }))
	}
}

func codeList(n int, item func(int) string) string {
	parts := make([]string, 0, n)
	for i := 0; i < n && i < 6; i++ {
		parts = append(parts, item(i))
	}
	sort.Strings(parts)
	s := strings.Join(parts, ", ")
	if n > 6 {
		s += fmt.Sprintf(", +%d more", n-6)
	}
	return s
}
