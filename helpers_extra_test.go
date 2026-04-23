package main

import (
	"errors"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"
)

func TestParseFlagErrorBranches(t *testing.T) {
	if got := parseFlagError(nil, "hint"); got != nil {
		t.Fatalf("expected nil for nil parse error, got %#v", got)
	}
	got := parseFlagError(errors.New("bad flag"), "use --help")
	if got == nil || got.Code != "usage" || got.Hint != "use --help" {
		t.Fatalf("unexpected parse flag error conversion: %#v", got)
	}
}

func TestParseFlagError_UnsupportedJSON(t *testing.T) {
	got := parseFlagError(errors.New("flag provided but not defined: -json"), "use --help")
	if got == nil {
		t.Fatalf("expected usage error for unsupported --json")
	}
	if got.Code != "usage" || got.Message != "--json is not supported by gh review-comments" {
		t.Fatalf("unexpected unsupported --json error: %#v", got)
	}
	if got.Hint != "remove --json; this extension always outputs TOON" {
		t.Fatalf("unexpected unsupported --json hint: %#v", got)
	}
	if got.Details == nil || got.Details["unsupportedFlag"] != "--json" {
		t.Fatalf("unexpected unsupported --json details: %#v", got.Details)
	}
}

func TestRepoStringBranches(t *testing.T) {
	if got := repoString(repository.Repository{}); got != "" {
		t.Fatalf("expected empty repo string for empty repo, got %q", got)
	}
	if got := repoString(repository.Repository{Owner: "octo", Name: "repo"}); got != "octo/repo" {
		t.Fatalf("expected owner/name repo string, got %q", got)
	}
}

func TestIntFlagStringBranches(t *testing.T) {
	var f intFlag
	if got := f.String(); got != "" {
		t.Fatalf("expected empty string for unset intFlag, got %q", got)
	}
	f.set = true
	f.value = 42
	if got := f.String(); got != "42" {
		t.Fatalf("expected numeric string for set intFlag, got %q", got)
	}
}
