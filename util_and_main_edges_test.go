package main

import (
	"os"
	"strings"
	"testing"
)

func TestTruncateRunesBranches(t *testing.T) {
	if got := truncateRunes("abcdef", nil); got != "abcdef" {
		t.Fatalf("expected unchanged string for nil limit, got %q", got)
	}
	zero := 0
	if got := truncateRunes("abcdef", &zero); got != "" {
		t.Fatalf("expected empty string for zero limit, got %q", got)
	}
	neg := -1
	if got := truncateRunes("abcdef", &neg); got != "abcdef" {
		t.Fatalf("expected unchanged string for negative limit, got %q", got)
	}
	one := 1
	if got := truncateRunes("abcdef", &one); got != "…" {
		t.Fatalf("expected ellipsis for limit 1, got %q", got)
	}
	three := 3
	if got := truncateRunes("abcdef", &three); got != "ab…" {
		t.Fatalf("expected truncation with ellipsis, got %q", got)
	}
}

func TestAuthorMatchesEmptyFilters(t *testing.T) {
	// Empty filters should match ALL authors (no filtering).
	if !authorMatches(map[string]struct{}{}, "bot") {
		t.Fatalf("expected true with empty filters (match all)")
	}
	if !authorMatches(map[string]struct{}{}, "anyone") {
		t.Fatalf("expected true with empty filters for any author")
	}
	if !authorMatches(authorsSet([]string{"Bot"}), "bot") {
		t.Fatalf("expected canonicalized match")
	}
	if !authorMatches(authorsSet([]string{"copilot-pull-request-reviewer"}), "copilot-pull-request-reviewer[bot]") {
		t.Fatalf("expected [bot]-suffixed candidate to match canonical filter")
	}
	if !authorMatches(authorsSet([]string{"copilot-pull-request-reviewer[bot]"}), "copilot-pull-request-reviewer") {
		t.Fatalf("expected canonical candidate to match [bot]-suffixed filter")
	}
}

func TestNormalizeAuthorsNoDefault(t *testing.T) {
	// When no authors are provided, normalizeAuthors should return empty
	// (no default author injected).
	got := normalizeAuthors(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty authors when none provided, got %v", got)
	}
	got = normalizeAuthors([]string{})
	if len(got) != 0 {
		t.Fatalf("expected empty authors for empty slice, got %v", got)
	}
}

func TestParsePRRefInvalidNumberInURL(t *testing.T) {
	if _, _, _, err := parsePRRef("https://github.com/octo/repo/pull/0"); err == nil {
		t.Fatalf("expected invalid PR number error")
	}
}

func TestReadBodyFromFileBranches(t *testing.T) {
	tmp := t.TempDir() + "/body.txt"
	if err := os.WriteFile(tmp, []byte("from-file"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if got, err := readBodyFromFile(tmp); err != nil || got != "from-file" {
		t.Fatalf("unexpected file body result: got=%q err=%#v", got, err)
	}
	if _, err := readBodyFromFile(t.TempDir() + "/missing.txt"); err == nil {
		t.Fatalf("expected usage error for missing file")
	}

	emptyFile := t.TempDir() + "/empty.txt"
	if err := os.WriteFile(emptyFile, []byte("   "), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if _, err := readBodyFromFile(emptyFile); err == nil {
		t.Fatalf("expected usage error for empty file body")
	}
}

func TestNewDefaultAppWiresDependencies(t *testing.T) {
	a := newDefaultApp(strings.NewReader(""), &strings.Builder{}, &strings.Builder{})
	if a == nil || a.ghapi == nil || a.repoCurrent == nil || a.repoParse == nil || a.ghExec == nil || a.gitSpiceLog == nil {
		t.Fatalf("newDefaultApp returned incomplete dependency wiring: %#v", a)
	}
	if repo, err := a.repoParse("octo/repo"); err != nil || repo.Owner != "octo" || repo.Name != "repo" {
		t.Fatalf("unexpected repo parse result: repo=%#v err=%v", repo, err)
	}
	_, _ = a.repoCurrent()
	_, _, _ = a.ghExec("--version")
	_, _, _ = a.gitSpiceLog()
}
