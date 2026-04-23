package main

import (
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/repository"
)

func TestRunMainHelpPath(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := runMain([]string{"--help"}, strings.NewReader(""), stdout, stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "gh review-comments") {
		t.Fatalf("expected root help output, got: %s", out)
	}
	if strings.HasPrefix(out, "ok: ") {
		t.Fatalf("help output must not be TOON payload: %s", out)
	}
}

func TestSubcommandHelpPaths(t *testing.T) {
	cases := []struct {
		args   []string
		expect string
	}{
		{args: []string{"list", "--help"}, expect: "gh review-comments list"},
		{args: []string{"reply", "--help"}, expect: "gh review-comments reply"},
		{args: []string{"resolve", "--help"}, expect: "gh review-comments resolve"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			app, stdout, _ := newTestApp(newFakeGitHub())
			code := app.run(tc.args)
			if code != 0 {
				t.Fatalf("expected exit code 0, got %d", code)
			}
			out := stdout.String()
			if !strings.Contains(out, tc.expect) {
				t.Fatalf("expected help output containing %q, got: %s", tc.expect, out)
			}
			if strings.HasPrefix(out, "ok: ") {
				t.Fatalf("help output must not be TOON payload: %s", out)
			}
		})
	}
}

func TestScanPRFormatsData(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 7)] = ghPR{Number: 7, Title: "Current PR", URL: "https://github.com/octo/repo/pull/7"}
	fake.threads[keyPR("octo", "repo", 7)] = []ghThread{{
		ID:         "T1",
		Path:       "a.go",
		Line:       10,
		Side:       "RIGHT",
		IsOutdated: false,
		IsResolved: false,
		Comments: []ghThreadComment{
			{ID: 5, Author: "copilot-pull-request-reviewer", Body: "body", CreatedAt: "2025-01-01T00:00:00Z"},
		},
	}}
	fake.reviews[keyPR("octo", "repo", 7)] = []ghReviewBody{{
		ReviewID:    11,
		Author:      "copilot-pull-request-reviewer",
		SubmittedAt: "2025-01-01T00:00:00Z",
		Body:        "<details>\n<summary>Comments suppressed due to low confidence (1)</summary>\n\n**x.go:1**\n* suppressed\n</details>",
	}}

	app, _, _ := newTestApp(fake)
	out, appErr := app.scanPR(
		repository.Repository{Owner: "octo", Name: "repo", Host: "github.com"},
		7,
		authorsSet([]string{"copilot-pull-request-reviewer"}),
		nil,
	)
	if appErr != nil {
		t.Fatalf("unexpected error: %#v", appErr)
	}
	if out.Number != 7 || out.Title != "Current PR" || len(out.Threads) != 1 || len(out.Suppressed) != 1 {
		t.Fatalf("unexpected scan output: %#v", out)
	}
}

func TestScanPRClassifiesAPIErrors(t *testing.T) {
	fake := newFakeGitHub()
	fake.errByKey["GetPR:"+keyPR("octo", "repo", 7)] = errNotFound
	app, _, _ := newTestApp(fake)

	_, appErr := app.scanPR(
		repository.Repository{Owner: "octo", Name: "repo", Host: "github.com"},
		7,
		authorsSet([]string{"copilot-pull-request-reviewer"}),
		nil,
	)
	if appErr == nil {
		t.Fatalf("expected error")
	}
	if appErr.Code != "notfound" {
		t.Fatalf("expected notfound code, got %#v", appErr)
	}
}

func TestNextLinkAndParsePRNumberHelpers(t *testing.T) {
	next, ok := nextLink(`<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=3>; rel="last"`)
	if !ok || next != "https://api.github.com/x?page=2" {
		t.Fatalf("unexpected next link parse: %q %v", next, ok)
	}
	if _, ok := nextLink(`<https://api.github.com/x?page=3>; rel="last"`); ok {
		t.Fatalf("expected no next link")
	}

	n, err := parsePRNumberFromURL("https://api.github.com/repos/octo/repo/pulls/12/comments")
	if err != nil || n != 12 {
		t.Fatalf("unexpected PR number parse: n=%d err=%v", n, err)
	}
	if _, err := parsePRNumberFromURL("https://api.github.com/repos/octo/repo/issues/12"); err == nil {
		t.Fatalf("expected invalid pull_request_url parse error")
	}
}

func TestBatchConvertersWithoutPagination(t *testing.T) {
	client := &gitHubClient{}
	threads, err := client.convertBatchThreads("octo", "repo", 7, listBatchThreadConn{
		Nodes: []listBatchThreadNode{
			{
				ID:         "T1",
				Path:       "a.go",
				Line:       nil,
				DiffSide:   nil,
				IsOutdated: false,
				IsResolved: false,
				Comments: listBatchCommentConn{
					Nodes: []listBatchCommentNode{
						{DatabaseID: nil, Body: "skip"},
						{DatabaseID: new(int64(9)), Body: "c9", CreatedAt: "2025-01-01T00:00:00Z"},
						{DatabaseID: new(int64(1)), Body: "c1", CreatedAt: "2025-01-01T00:00:00Z"},
					},
					PageInfo: listBatchPageInfo{HasNextPage: false},
				},
			},
		},
		PageInfo: listBatchPageInfo{HasNextPage: false},
	})
	if err != nil {
		t.Fatalf("unexpected convertBatchThreads error: %v", err)
	}
	if len(threads) != 1 || len(threads[0].Comments) != 2 {
		t.Fatalf("unexpected thread conversion output: %#v", threads)
	}
	if threads[0].Comments[0].ID != 1 || threads[0].Comments[1].ID != 9 {
		t.Fatalf("expected deterministic tie-order by ID, got %#v", threads[0].Comments)
	}

	reviews, err := client.convertBatchReviews("octo", "repo", 7, listBatchReviewConn{
		Nodes: []listBatchReviewNode{
			{DatabaseID: new(int64(1)), Body: "  ", SubmittedAt: "2025-01-01T00:00:00Z"},
			{DatabaseID: new(int64(2)), Body: "use", SubmittedAt: "2025-02-01T00:00:00Z"},
		},
		PageInfo: listBatchPageInfo{HasNextPage: false},
	})
	if err != nil {
		t.Fatalf("unexpected convertBatchReviews error: %v", err)
	}
	if len(reviews) != 1 || reviews[0].ReviewID != 2 || reviews[0].Body != "use" {
		t.Fatalf("unexpected review conversion output: %#v", reviews)
	}
}
