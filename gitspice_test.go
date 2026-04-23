package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestRunGitSpiceLogCommandWiring(t *testing.T) {
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

	stdout, stderr, err := runGitSpiceLog()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotName != "git-spice" || !reflect.DeepEqual(gotArgs, []string{"log", "short", "--json"}) {
		t.Fatalf("unexpected command invocation: %q %#v", gotName, gotArgs)
	}
	if stdout != "out" || stderr != "err" {
		t.Fatalf("unexpected outputs: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestRunGitSpiceLogPropagatesCommandError(t *testing.T) {
	old := runExternalCommand
	t.Cleanup(func() { runExternalCommand = old })

	runExternalCommand = func(name string, args ...string) (string, string, error) {
		return "", "boom", fmt.Errorf("exit status 1")
	}
	_, stderr, err := runGitSpiceLog()
	if err == nil {
		t.Fatalf("expected command error")
	}
	if stderr != "boom" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestDiscoverStackPRs_HappyPath(t *testing.T) {
	run := func() (string, string, error) {
		return "{\"change\":{\"id\":\"#101\"}}\n{\"change\":{\"id\":\"#102\"}}", "", nil
	}
	prs, warnings, err := discoverStackPRs(run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if !reflect.DeepEqual(prs, []int{101, 102}) {
		t.Fatalf("unexpected PRs: %#v", prs)
	}
}

func TestDiscoverStackPRs_NoisyJSONLines(t *testing.T) {
	run := func() (string, string, error) {
		return "\nnot-json\n{\"change\":{\"id\":\"#103\"}}\n{\"change\":{\"id\":\"abc\"}}", "", nil
	}
	prs, warnings, err := discoverStackPRs(run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(prs, []int{103}) {
		t.Fatalf("unexpected PRs: %#v", prs)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %#v", warnings)
	}
}

func TestDiscoverStackPRs_ReturnsErrorOnNonZero(t *testing.T) {
	run := func() (string, string, error) {
		return "", "not tracked", fmt.Errorf("exit status 1")
	}
	prs, warnings, err := discoverStackPRs(run)
	if err == nil {
		t.Fatalf("expected git-spice failure error")
	}
	if len(prs) != 0 {
		t.Fatalf("expected no PRs, got %#v", prs)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected warning on git-spice failure, got %#v", warnings)
	}
}

func TestDiscoverStackPRs_ReturnsErrorOnAnyFailure(t *testing.T) {
	run := func() (string, string, error) {
		return "", "some other failure", fmt.Errorf("boom")
	}
	prs, warnings, err := discoverStackPRs(run)
	if err == nil {
		t.Fatalf("expected git-spice failure error")
	}
	if len(prs) != 0 {
		t.Fatalf("expected no PRs, got %#v", prs)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected warning on git-spice failure, got %#v", warnings)
	}
}
