package main

import (
	"os"
	"strings"
	"testing"
)

func TestStdoutPurity_NonHelpSuccessAndFailure(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 7)] = ghPR{Number: 7, Title: "Current PR", URL: "https://github.com/octo/repo/pull/7"}
	fake.threads[keyPR("octo", "repo", 7)] = []ghThread{{
		ID:         "T1",
		Path:       "a.go",
		Line:       10,
		Side:       "RIGHT",
		IsOutdated: false,
		Comments: []ghThreadComment{{
			ID: 1, Author: "copilot-pull-request-reviewer", Body: "body", CreatedAt: "2025-01-01T00:00:00Z",
		}},
	}}
	app, stdout, _ := newTestApp(fake)

	code := app.run([]string{"list"})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if stdout.String() == "" {
		t.Fatalf("expected stdout TOON")
	}
	if !strings.HasPrefix(stdout.String(), "ok: true\n") {
		t.Fatalf("stdout is not a TOON success payload: %s", stdout.String())
	}
	if strings.Count(stdout.String(), "\nok: ") != 0 {
		t.Fatalf("stdout appears to contain multiple TOON docs: %s", stdout.String())
	}

	app2, stdout2, _ := newTestApp(newFakeGitHub())
	code = app2.run([]string{"reply"})
	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	if !strings.HasPrefix(stdout2.String(), "ok: false\n") {
		t.Fatalf("stdout is not TOON on failure: %s", stdout2.String())
	}
}

func TestListOrderingAndDeterminism(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 2)] = ghPR{Number: 2, Title: "Zeta Title", URL: "https://github.com/octo/repo/pull/2"}
	fake.prs[keyPR("octo", "repo", 1)] = ghPR{Number: 1, Title: "Alpha Title", URL: "https://github.com/octo/repo/pull/1"}
	fake.threads[keyPR("octo", "repo", 2)] = []ghThread{
		{ID: "T2", Path: "z.go", Line: 30, Side: "RIGHT", Comments: []ghThreadComment{{ID: 201, Author: "zeta", Body: "ignored", CreatedAt: "2025-01-01T00:00:00Z"}, {ID: 200, Author: "alpha", Body: "keep2", CreatedAt: "2024-01-01T00:00:00Z"}}},
		{ID: "T1", Path: "a.go", Line: 10, Side: "LEFT", Comments: []ghThreadComment{{ID: 101, Author: "alpha", Body: "keep1", CreatedAt: "2024-01-01T00:00:00Z"}, {ID: 102, Author: "you", Body: "reply", CreatedAt: "2024-01-02T00:00:00Z"}}},
	}
	fake.reviews[keyPR("octo", "repo", 2)] = []ghReviewBody{{ReviewID: 11, Body: `<details>
<summary>Comments suppressed due to low confidence (1)</summary>

**x.go:99**
* suppressed reason

go
code

</details>`}}
	fake.threads[keyPR("octo", "repo", 1)] = []ghThread{
		{ID: "R1", Path: "m.go", Line: 1, Side: "RIGHT", Comments: []ghThreadComment{{ID: 301, Author: "alpha", Body: "p1", CreatedAt: "2023-01-01T00:00:00Z"}}},
	}

	app, stdout, _ := newTestApp(fake)
	code := app.run([]string{"list", "--repo", "octo/repo", "--pr", "2", "--pr", "1", "--author", "zeta", "--author", "alpha", "--author", "zeta"})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}

	if !strings.Contains(stdout.String(), "authors[#2]: alpha,zeta") {
		t.Fatalf("authors not sorted/deduped: %s", stdout.String())
	}
	pr2 := strings.Index(stdout.String(), "- number: 2")
	pr1 := strings.Index(stdout.String(), "- number: 1")
	if pr2 < 0 || pr1 < 0 || pr2 > pr1 {
		t.Fatalf("PR ordering mismatch: %s", stdout.String())
	}
	pathA := strings.Index(stdout.String(), "- path: a.go")
	pathZ := strings.Index(stdout.String(), "- path: z.go")
	if pathA < 0 || pathZ < 0 || pathA > pathZ {
		t.Fatalf("thread ordering mismatch: %s", stdout.String())
	}
	c1 := strings.Index(stdout.String(), "201,zeta,ignored")
	c2 := strings.Index(stdout.String(), "200,alpha,keep2")
	if c1 < 0 || c2 < 0 || c1 > c2 {
		t.Fatalf("comment ordering mismatch: %s", stdout.String())
	}
}

func TestListSuppressedUsesLatestRelevantReviewOnly(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 7)] = ghPR{Number: 7, Title: "Current PR", URL: "https://github.com/octo/repo/pull/7"}
	fake.threads[keyPR("octo", "repo", 7)] = []ghThread{}
	fake.reviews[keyPR("octo", "repo", 7)] = []ghReviewBody{
		{
			ReviewID:    101,
			Author:      "copilot-pull-request-reviewer",
			SubmittedAt: "2025-01-01T00:00:00Z",
			Body:        "<details>\n<summary>Comments suppressed due to low confidence (1)</summary>\n\n**old.go:1**\n* old item\n</details>",
		},
		{
			ReviewID:    202,
			Author:      "copilot-pull-request-reviewer",
			SubmittedAt: "2025-02-01T00:00:00Z",
			Body:        "<details>\n<summary>Comments suppressed due to low confidence (1)</summary>\n\n**new.go:2**\n* new item\n</details>",
		},
		{
			ReviewID:    303,
			Author:      "someone-else",
			SubmittedAt: "2025-03-01T00:00:00Z",
			Body:        "<details>\n<summary>Comments suppressed due to low confidence (1)</summary>\n\n**other.go:3**\n* other item\n</details>",
		},
	}

	// With an explicit --author filter, only matching reviews are considered.
	app, stdout, _ := newTestApp(fake)
	if code := app.run([]string{"list", "--author", "copilot-pull-request-reviewer"}); code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "suppressed[#1]{path,line,reviewId,index,body}:") {
		t.Fatalf("expected exactly one suppressed item from latest relevant review: %s", out)
	}
	if !strings.Contains(out, "new.go,2,202,1,new item") {
		t.Fatalf("expected suppressed from latest relevant review, got: %s", out)
	}
	if strings.Contains(out, "old.go,1,101,1,old item") || strings.Contains(out, "other.go,3,303,1,other item") {
		t.Fatalf("expected older/unfiltered suppressed entries to be excluded: %s", out)
	}

	// Without --author, all reviews are considered so the latest (someone-else) wins.
	app2, stdout2, _ := newTestApp(fake)
	if code := app2.run([]string{"list"}); code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	out2 := stdout2.String()
	if !strings.Contains(out2, "other.go,3,303,1,other item") {
		t.Fatalf("expected suppressed from latest review (any author) when no --author, got: %s", out2)
	}
}

func TestListBatchesPerRepo(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 2)] = ghPR{Number: 2, Title: "Two", URL: "https://github.com/octo/repo/pull/2"}
	fake.prs[keyPR("octo", "repo", 1)] = ghPR{Number: 1, Title: "One", URL: "https://github.com/octo/repo/pull/1"}
	fake.threads[keyPR("octo", "repo", 2)] = []ghThread{}
	fake.threads[keyPR("octo", "repo", 1)] = []ghThread{}

	app, _, _ := newTestApp(fake)
	if code := app.run([]string{"list", "--repo", "octo/repo", "--pr", "2", "--pr", "1"}); code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	if fake.listPRDataCalls != 1 {
		t.Fatalf("expected one batched list call, got %d", fake.listPRDataCalls)
	}
	if fake.getPRCalls != 0 || fake.getThreadsCalls != 0 || fake.getReviewBodyCalls != 0 {
		t.Fatalf("expected no per-PR scan fallback calls, got GetPR=%d GetReviewThreads=%d GetReviewBodies=%d", fake.getPRCalls, fake.getThreadsCalls, fake.getReviewBodyCalls)
	}
}

func TestListNoAuthorFlagReturnsAllAuthors(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 7)] = ghPR{Number: 7, Title: "Current PR", URL: "https://github.com/octo/repo/pull/7"}
	fake.threads[keyPR("octo", "repo", 7)] = []ghThread{
		{
			ID:   "T1",
			Path: "a.go",
			Line: 10,
			Comments: []ghThreadComment{
				{ID: 55, Author: "copilot-pull-request-reviewer", Body: "bot comment", CreatedAt: "2026-03-01T00:00:00Z"},
			},
		},
		{
			ID:   "T2",
			Path: "b.go",
			Line: 20,
			Comments: []ghThreadComment{
				{ID: 66, Author: "human-reviewer", Body: "human comment", CreatedAt: "2026-03-02T00:00:00Z"},
			},
		},
	}
	fake.reviews[keyPR("octo", "repo", 7)] = []ghReviewBody{
		{
			ReviewID:    999,
			Author:      "copilot-pull-request-reviewer",
			SubmittedAt: "2026-03-01T00:00:00Z",
			Body:        "<details>\n<summary>Comments suppressed due to low confidence (1)</summary>\n\n**x.go:1**\n* suppressed from bot\n</details>",
		},
		{
			ReviewID:    1000,
			Author:      "human-reviewer",
			SubmittedAt: "2026-03-02T00:00:00Z",
			Body:        "<details>\n<summary>Comments suppressed due to low confidence (1)</summary>\n\n**y.go:2**\n* suppressed from human\n</details>",
		},
	}

	app, stdout, _ := newTestApp(fake)
	if code := app.run([]string{"list"}); code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	out := stdout.String()
	// Without --author, both authors' threads should appear.
	if !strings.Contains(out, "threads[#2]:") {
		t.Fatalf("expected 2 threads from all authors, got: %s", out)
	}
	if !strings.Contains(out, "bot comment") || !strings.Contains(out, "human comment") {
		t.Fatalf("expected comments from both authors, got: %s", out)
	}
	// Authors list should be empty (no filter applied).
	if !strings.Contains(out, "authors[#0]:") {
		t.Fatalf("expected empty authors list when --author not passed, got: %s", out)
	}
	// Suppressed should come from the latest review across all authors.
	if !strings.Contains(out, "y.go,2,1000,1,suppressed from human") {
		t.Fatalf("expected suppressed from latest review (any author), got: %s", out)
	}
}

func TestListGoldenOutput(t *testing.T) {
	fake := newFakeGitHub()
	fake.prs[keyPR("octo", "repo", 7)] = ghPR{Number: 7, Title: "Current PR", URL: "https://github.com/octo/repo/pull/7"}
	fake.threads[keyPR("octo", "repo", 7)] = []ghThread{{
		ID:         "T1",
		Path:       "a.go",
		Line:       10,
		Side:       "RIGHT",
		IsOutdated: true,
		Comments: []ghThreadComment{
			{ID: 10, Author: "copilot-pull-request-reviewer", Body: "Long body here", CreatedAt: "2024-01-01T00:00:00Z"},
			{ID: 11, Author: "you", Body: "Ack", CreatedAt: "2024-01-01T00:00:01Z"},
		},
	}}
	fake.reviews[keyPR("octo", "repo", 7)] = []ghReviewBody{{
		ReviewID:    77,
		Author:      "copilot-pull-request-reviewer",
		SubmittedAt: "2024-01-01T00:00:00Z",
		Body:        "<details>\n<summary>Comments suppressed due to low confidence (1)</summary>\n\n**main.go:42**\n* Consider renaming variable\n\n```go\nx := 1\n```\n</details>",
	}}
	app, stdout, _ := newTestApp(fake)
	if code := app.run([]string{"list", "--max-body", "6"}); code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}

	golden, err := os.ReadFile("testdata/list_output.golden.toon")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := strings.TrimSuffix(stdout.String(), "\n")
	want := strings.TrimSuffix(string(golden), "\n")
	if got != want {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestListRejectsMixedRepositoriesInPRFlags(t *testing.T) {
	fake := newFakeGitHub()
	app, stdout, _ := newTestApp(fake)

	code := app.run([]string{
		"list",
		"--pr", "https://github.com/octo/repo/pull/1",
		"--pr", "https://github.com/other/repo/pull/2",
	})
	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "code: usage") || !strings.Contains(out, "mixed repositories in --pr values") {
		t.Fatalf("expected usage error for mixed repositories, got: %s", out)
	}
	if fake.listPRDataCalls != 0 {
		t.Fatalf("expected no API calls on mixed repository args, got %d", fake.listPRDataCalls)
	}
}

func TestListMaxBodyStrictIntegerParsing(t *testing.T) {
	cases := []string{"7xyz", "2.9"}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			app, stdout, _ := newTestApp(newFakeGitHub())
			code := app.run([]string{"list", "--repo", "octo/repo", "--pr", "1", "--max-body", value})
			if code != 2 {
				t.Fatalf("expected usage exit code 2, got %d", code)
			}
			out := stdout.String()
			if !strings.Contains(out, "code: usage") || !strings.Contains(out, "invalid arguments") {
				t.Fatalf("expected invalid arguments usage error, got: %s", out)
			}
		})
	}
}

func TestInvalidRepoFormatUsesUsageExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	msgFile := tmpDir + "/body.txt"
	if err := os.WriteFile(msgFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	commands := [][]string{
		{"list", "--repo", "bad", "--pr", "1"},
		{"reply", "99", "--message-file", msgFile, "--repo", "bad"},
		{"resolve", "99", "--repo", "bad"},
	}
	for _, cmd := range commands {
		t.Run(strings.Join(cmd[:1], " "), func(t *testing.T) {
			app, stdout, _ := newTestApp(newFakeGitHub())
			code := app.run(cmd)
			if code != 2 {
				t.Fatalf("expected usage exit code 2, got %d", code)
			}
			out := stdout.String()
			if !strings.Contains(out, "code: repo") || !strings.Contains(out, "invalid repository") {
				t.Fatalf("expected invalid repository error, got: %s", out)
			}
		})
	}
}

func TestReplyAndResolveHappyPaths(t *testing.T) {
	fake := newFakeGitHub()
	fake.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}
	fake.threadByC["octo/repo#12@99"] = "THREAD_1"

	tmpDir := t.TempDir()
	msgFile := tmpDir + "/reply.md"
	if err := os.WriteFile(msgFile, []byte("thanks"), 0o644); err != nil {
		t.Fatalf("write message file: %v", err)
	}

	// reply resolves by default
	app, stdout, _ := newTestApp(fake)
	if code := app.run([]string{"reply", "99", "--message-file", msgFile}); code != 0 {
		t.Fatalf("reply unexpected exit code: %d", code)
	}
	if len(fake.replies) != 1 {
		t.Fatalf("expected one reply call, got %d", len(fake.replies))
	}
	if fake.replies[0].Body != "thanks" || fake.replies[0].PR != 12 {
		t.Fatalf("unexpected reply call: %#v", fake.replies[0])
	}
	if !strings.Contains(stdout.String(), "action: reply") {
		t.Fatalf("expected reply payload, got: %s", stdout.String())
	}
	if len(fake.resolved) != 1 || fake.resolved[0] != "THREAD_1" {
		t.Fatalf("expected resolved thread THREAD_1, got %#v", fake.resolved)
	}

	// standalone resolve still works
	fake2 := newFakeGitHub()
	fake2.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}
	fake2.threadByC["octo/repo#12@99"] = "THREAD_1"
	app2, stdout2, _ := newTestApp(fake2)
	if code := app2.run([]string{"resolve", "99"}); code != 0 {
		t.Fatalf("resolve unexpected exit code: %d", code)
	}
	if len(fake2.resolved) != 1 || fake2.resolved[0] != "THREAD_1" {
		t.Fatalf("expected resolved thread THREAD_1, got %#v", fake2.resolved)
	}
	if !strings.Contains(stdout2.String(), "action: resolve") {
		t.Fatalf("expected resolve payload, got: %s", stdout2.String())
	}
}

func TestReplySupportsMessageFile(t *testing.T) {
	fake := newFakeGitHub()
	fake.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}

	tmpDir := t.TempDir()
	msgFile := tmpDir + "/reply.md"
	fileBody := "Addressed with `code ticks`.\n\n```go\nfmt.Println(\"ok\")\n```"
	if err := os.WriteFile(msgFile, []byte(fileBody), 0o644); err != nil {
		t.Fatalf("write message file: %v", err)
	}

	app, _, _ := newTestApp(fake)
	if code := app.run([]string{"reply", "99", "--message-file", msgFile, "--no-resolve"}); code != 0 {
		t.Fatalf("reply (--message-file) unexpected exit code: %d", code)
	}
	if len(fake.replies) != 1 || fake.replies[0].Body != fileBody {
		t.Fatalf("unexpected file reply call: %#v", fake.replies)
	}
}

func TestReplyRejectsRemovedFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "-m", args: []string{"reply", "99", "-m", "inline"}},
		{name: "--message-stdin", args: []string{"reply", "99", "--message-stdin"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, stdout, _ := newTestApp(newFakeGitHub())
			code := app.run(tc.args)
			if code != 2 {
				t.Fatalf("expected usage exit code 2, got %d", code)
			}
			if !strings.Contains(stdout.String(), "code: usage") {
				t.Fatalf("expected usage failure, got: %s", stdout.String())
			}
		})
	}
}

func TestReplyRejectsLegacyBodyFileFlag(t *testing.T) {
	app, stdout, _ := newTestApp(newFakeGitHub())
	code := app.run([]string{"reply", "99", "--body-file", "body.txt"})
	if code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "invalid arguments") || !strings.Contains(out, "body-file") {
		t.Fatalf("expected invalid arguments mentioning --body-file, got: %s", out)
	}
}

func TestReplyRejectsEmptyBodyInput(t *testing.T) {
	tmpDir := t.TempDir()
	emptyBodyFile := tmpDir + "/empty.txt"
	if err := os.WriteFile(emptyBodyFile, []byte("  \n"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	cases := [][]string{
		{"reply", "99", "--message-file", emptyBodyFile},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, stdout, _ := newTestApp(newFakeGitHub())
			code := app.run(args)
			if code != 2 {
				t.Fatalf("expected usage exit code 2, got %d", code)
			}
			out := stdout.String()
			if !strings.Contains(out, "reply body must not be empty") {
				t.Fatalf("expected empty body validation error, got: %s", out)
			}
		})
	}
}
