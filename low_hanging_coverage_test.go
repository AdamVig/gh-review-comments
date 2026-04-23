package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type writeErrWriter struct{}

func (writeErrWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestListScopeStackFromGitSpice(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 101)] = ghPR{Number: 101, Title: "One", URL: "https://github.com/octo/repo/pull/101"}
	fake.prs[keyPR("octo", "repo", 102)] = ghPR{Number: 102, Title: "Two", URL: "https://github.com/octo/repo/pull/102"}
	fake.threads[keyPR("octo", "repo", 101)] = []ghThread{}
	fake.threads[keyPR("octo", "repo", 102)] = []ghThread{}

	app, stdout, _ := newTestApp(fake)
	app.gitSpiceLog = func() (string, string, error) {
		return "{\"change\":{\"id\":\"#101\"}}\n{\"change\":{\"id\":\"#102\"}}", "", nil
	}

	code := app.run([]string{"list-stack"})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "scope: stack") {
		t.Fatalf("expected stack scope, got: %s", out)
	}
	if !strings.Contains(out, "- number: 101") || !strings.Contains(out, "- number: 102") {
		t.Fatalf("expected stack PR entries, got: %s", out)
	}
}

func TestListRejectsScopeFlag(t *testing.T) {
	app, stdout, _ := newTestApp(newFakeGitHub())
	code := app.run([]string{"list", "--scope", "stack"})
	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "code: usage") {
		t.Fatalf("expected usage error, got: %s", out)
	}
}

func TestListCurrentPRErrorPaths(t *testing.T) {
	t.Run("gh exec fails", func(t *testing.T) {
		app, stdout, _ := newTestApp(newFakeGitHub())
		app.gitSpiceLog = func() (string, string, error) { return "", "", fmt.Errorf("git-spice failed") }
		app.ghExec = func(args ...string) (string, string, error) { return "", "boom", errors.New("boom") }

		code := app.run([]string{"list"})
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		out := stdout.String()
		if !strings.Contains(out, "code: repo") || !strings.Contains(out, "could not infer current pull request") {
			t.Fatalf("expected repo inference failure, got: %s", out)
		}
	})

	t.Run("gh exec returns invalid json", func(t *testing.T) {
		app, stdout, _ := newTestApp(newFakeGitHub())
		app.gitSpiceLog = func() (string, string, error) { return "", "", fmt.Errorf("git-spice failed") }
		app.ghExec = func(args ...string) (string, string, error) { return "{", "", nil }

		code := app.run([]string{"list"})
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
		out := stdout.String()
		if !strings.Contains(out, "code: parse") || !strings.Contains(out, "failed to parse current pull request") {
			t.Fatalf("expected parse failure, got: %s", out)
		}
	})
}

func TestIsNewerReviewBranches(t *testing.T) {
	if isNewerReview(
		ghReviewBody{SubmittedAt: "", ReviewID: 2},
		ghReviewBody{SubmittedAt: "2025-01-01T00:00:00Z", ReviewID: 1},
	) {
		t.Fatalf("empty SubmittedAt should not be newer")
	}
	if !isNewerReview(
		ghReviewBody{SubmittedAt: "2025-01-01T00:00:00Z", ReviewID: 1},
		ghReviewBody{SubmittedAt: "", ReviewID: 2},
	) {
		t.Fatalf("non-empty SubmittedAt should be newer than empty")
	}
	if !isNewerReview(
		ghReviewBody{SubmittedAt: "2025-01-01T00:00:00Z", ReviewID: 3},
		ghReviewBody{SubmittedAt: "2025-01-01T00:00:00Z", ReviewID: 2},
	) {
		t.Fatalf("higher ReviewID should win as tiebreaker")
	}
}

func TestParseInt64ArgInvalidValues(t *testing.T) {
	if _, err := parseInt64Arg("abc", "comment-id"); err == nil {
		t.Fatalf("expected parse error for non-numeric input")
	}
	if _, err := parseInt64Arg("0", "comment-id"); err == nil {
		t.Fatalf("expected validation error for non-positive input")
	}
}

func TestRunReturnsInternalErrorWhenStdoutWriteFails(t *testing.T) {
	app := &app{
		stdin:  strings.NewReader(""),
		stdout: writeErrWriter{},
		stderr: &strings.Builder{},
		ghapi:  newFakeGitHub(),
	}

	code := app.run([]string{"reply"})
	if code != 1 {
		t.Fatalf("expected internal exit code 1 when stdout write fails, got %d", code)
	}
}
