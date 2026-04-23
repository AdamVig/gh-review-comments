package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	exitSuccess = 0
	exitError   = 1
	exitUsage   = 2
)

type appError struct {
	Code    string
	Message string
	Hint    string
	Details map[string]any
	Exit    int
}

func newUsageError(message, hint string, details map[string]any) *appError {
	return &appError{Code: "usage", Message: message, Hint: hint, Details: details, Exit: exitUsage}
}

func truncateRunes(s string, maxBody *int) string {
	if maxBody == nil {
		return s
	}
	n := *maxBody
	if n == 0 {
		return ""
	}
	if n < 0 {
		return s
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}

func normalizeAuthors(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, a := range in {
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

func authorsSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in)*2)
	for _, a := range in {
		out[canonicalAuthorLogin(a)] = struct{}{}
	}
	return out
}

func canonicalAuthorLogin(login string) string {
	v := strings.ToLower(strings.TrimSpace(login))
	v = strings.TrimSuffix(v, "[bot]")
	return strings.TrimSpace(v)
}

func authorMatches(filters map[string]struct{}, candidate string) bool {
	if len(filters) == 0 {
		return true
	}
	_, ok := filters[canonicalAuthorLogin(candidate)]
	return ok
}

func parseInt64Arg(v, name string) (int64, *appError) {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, newUsageError(
			fmt.Sprintf("invalid %s", name),
			fmt.Sprintf("pass a numeric %s", name),
			map[string]any{"value": v},
		)
	}
	if n <= 0 {
		return 0, newUsageError(
			fmt.Sprintf("invalid %s", name),
			fmt.Sprintf("pass a positive %s", name),
			map[string]any{"value": v},
		)
	}
	return n, nil
}

type stringSliceFlag struct {
	items []string
}

func (s *stringSliceFlag) String() string { return "" }

func (s *stringSliceFlag) Set(v string) error {
	s.items = append(s.items, v)
	return nil
}

type intFlag struct {
	set   bool
	value int
}

func (f *intFlag) String() string {
	if !f.set {
		return ""
	}
	return fmt.Sprintf("%d", f.value)
}

func (f *intFlag) Set(v string) error {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fmt.Errorf("invalid integer: %w", err)
	}
	f.set = true
	f.value = n
	return nil
}
