package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"
)

func TestListPartialSuccessReturnsExitZero(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 1)] = ghPR{Number: 1, Title: "One", URL: "https://github.com/octo/repo/pull/1"}
	fake.threads[keyPR("octo", "repo", 1)] = []ghThread{}

	app, stdout, _ := newTestApp(fake)
	code := app.run([]string{"list", "--repo", "octo/repo", "--pr", "1", "--pr", "2"})
	if code != 0 {
		t.Fatalf("expected exit 0 for partial success, got %d", code)
	}
	out := stdout.String()
	if !strings.HasPrefix(out, "ok: true\n") {
		t.Fatalf("expected success payload, got: %s", out)
	}
	if !strings.Contains(out, "- number: 2") ||
		!strings.Contains(out, "code: notfound") ||
		!strings.Contains(out, "message: resource not found") ||
		!strings.Contains(out, "hint: verify repository and identifier") {
		t.Fatalf("expected per-PR notfound error entry, got: %s", out)
	}
}

func TestListBatchCallErrorFallsBackToPerPRScan(t *testing.T) {
	fake := newFakeGitHub()
	fake.errByKey["ListPRDataCall:octo/repo"] = errors.New("batch call failed")
	fake.prs[keyPR("octo", "repo", 7)] = ghPR{Number: 7, Title: "Current PR", URL: "https://github.com/octo/repo/pull/7"}
	fake.threads[keyPR("octo", "repo", 7)] = []ghThread{}
	fake.reviews[keyPR("octo", "repo", 7)] = []ghReviewBody{}

	app, stdout, _ := newTestApp(fake)
	code := app.run([]string{"list", "--repo", "octo/repo", "--pr", "7"})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (out=%s)", code, stdout.String())
	}
	if fake.listPRDataCalls != 1 {
		t.Fatalf("expected batch call once, got %d", fake.listPRDataCalls)
	}
	if fake.getPRCalls == 0 || fake.getThreadsCalls == 0 || fake.getReviewBodyCalls == 0 {
		t.Fatalf("expected fallback per-PR calls, got GetPR=%d GetReviewThreads=%d GetReviewBodies=%d", fake.getPRCalls, fake.getThreadsCalls, fake.getReviewBodyCalls)
	}
}

func TestListNegativeMaxBodyRejected(t *testing.T) {
	app, stdout, _ := newTestApp(newFakeGitHub())
	code := app.run([]string{"list", "--repo", "octo/repo", "--pr", "1", "--max-body", "-1"})
	if code != 2 {
		t.Fatalf("expected usage exit 2, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "invalid --max-body") {
		t.Fatalf("expected invalid --max-body error, got: %s", out)
	}
}

func TestListExplicitPRWithoutRepoInferenceFailure(t *testing.T) {
	app, stdout, _ := newTestApp(newFakeGitHub())
	app.repoCurrent = func() (repository.Repository, error) {
		return repository.Repository{}, errors.New("no repo")
	}
	code := app.run([]string{"list", "--pr", "7"})
	if code != 1 {
		t.Fatalf("expected runtime exit 1, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "code: repo") || !strings.Contains(out, "failed to scan all pull requests") {
		t.Fatalf("expected repo failure payload, got: %s", out)
	}
}

func TestListScopeStackRequiresDiscoverableStack(t *testing.T) {
	app, stdout, _ := newTestApp(newFakeGitHub())
	app.gitSpiceLog = func() (string, string, error) {
		return "", "not tracked", errors.New("exit status 1")
	}

	code := app.run([]string{"list-stack"})
	if code != 1 {
		t.Fatalf("expected runtime exit 1, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "code: repo") || !strings.Contains(out, "could not infer stack pull requests") {
		t.Fatalf("expected stack inference failure payload, got: %s", out)
	}
}

func TestListScopeStackNoPRsFound(t *testing.T) {
	app, stdout, _ := newTestApp(newFakeGitHub())
	app.gitSpiceLog = func() (string, string, error) {
		return "", "", nil
	}

	code := app.run([]string{"list-stack"})
	if code != 1 {
		t.Fatalf("expected runtime exit 1, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "code: repo") || !strings.Contains(out, "no stack pull requests found") {
		t.Fatalf("expected empty stack failure payload, got: %s", out)
	}
}
