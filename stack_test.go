package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRunGHStackViewCommandWiring(t *testing.T) {
	old := runExternalCommand
	t.Cleanup(func() { runExternalCommand = old })

	var (
		gotName string
		gotArgs []string
	)
	runExternalCommand = func(name string, args ...string) (string, string, error) {
		gotName = name
		gotArgs = append([]string{}, args...)
		return "out", "err", nil
	}

	stdout, stderr, err := runGHStackView()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "gh" || !reflect.DeepEqual(gotArgs, []string{"stack", "view", "--json"}) {
		t.Fatalf("unexpected command invocation: %q %#v", gotName, gotArgs)
	}
	if stdout != "out" || stderr != "err" {
		t.Fatalf("unexpected outputs: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestRunGHStackViewPropagatesCommandError(t *testing.T) {
	old := runExternalCommand
	t.Cleanup(func() { runExternalCommand = old })

	runExternalCommand = func(name string, args ...string) (string, string, error) {
		return "", "boom", fmt.Errorf("exit status 1")
	}
	_, stderr, err := runGHStackView()
	if err == nil {
		t.Fatalf("expected command error")
	}
	if stderr != "boom" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestDiscoverStackPRs(t *testing.T) {
	stdout := `{
		"trunk": "main",
		"currentBranch": "feature-two",
		"branches": [
			{"name": "merged-branch", "isMerged": true, "pr": {"number": 100, "state": "OPEN"}},
			{"name": "merged-pr", "isMerged": false, "pr": {"number": 101, "state": "MERGED"}},
			{"name": "unpushed", "pr": null},
			{"name": "feature-two", "pr": {"number": 102, "state": "OPEN"}}
		]
	}`
	run := func() (string, string, error) { return stdout, "", nil }

	prs, err := discoverStackPRs(run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(prs, []int{102}) {
		t.Fatalf("unexpected PRs: %#v", prs)
	}
}

func TestDiscoverStackPRsRejectsInvalidJSON(t *testing.T) {
	run := func() (string, string, error) { return "not-json", "", nil }

	prs, err := discoverStackPRs(run)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if prs != nil {
		t.Fatalf("expected no PRs, got %#v", prs)
	}
	if !strings.Contains(err.Error(), "failed to parse gh stack view --json output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscoverStackPRsReturnsCommandError(t *testing.T) {
	run := func() (string, string, error) {
		return "", "not in a stack", fmt.Errorf("exit status 2")
	}

	prs, err := discoverStackPRs(run)
	if err == nil {
		t.Fatal("expected command error")
	}
	if prs != nil {
		t.Fatalf("expected no PRs, got %#v", prs)
	}
	if !strings.Contains(err.Error(), "gh stack view --json failed (not in a stack)") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDiscoverStackPRsReturnsCommandErrorWithoutStderr(t *testing.T) {
	run := func() (string, string, error) { return "", "", fmt.Errorf("boom") }

	_, err := discoverStackPRs(run)
	if err == nil || !strings.Contains(err.Error(), "gh stack view --json failed: boom") {
		t.Fatalf("unexpected error: %v", err)
	}
}
