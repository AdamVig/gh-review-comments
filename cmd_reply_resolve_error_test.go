package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestResolveUsageAndErrorPaths(t *testing.T) {
	t.Run("missing comment id", func(t *testing.T) {
		app, stdout, _ := newTestApp(newFakeGitHub())
		code := app.run([]string{"resolve"})
		if code != 2 {
			t.Fatalf("expected usage exit code 2, got %d", code)
		}
		if !strings.Contains(stdout.String(), "missing comment-id") {
			t.Fatalf("expected usage message, got: %s", stdout.String())
		}
	})

	t.Run("find thread not found", func(t *testing.T) {
		fake := newFakeGitHub()
		fake.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}

		app, stdout, _ := newTestApp(fake)
		code := app.run([]string{"resolve", "99"})
		if code != 1 {
			t.Fatalf("expected runtime exit code 1, got %d", code)
		}
		if !strings.Contains(stdout.String(), "code: notfound") {
			t.Fatalf("expected notfound payload, got: %s", stdout.String())
		}
	})

	t.Run("resolve thread internal error", func(t *testing.T) {
		fake := newFakeGitHub()
		fake.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}
		fake.threadByC["octo/repo#12@99"] = "THREAD_1"
		fake.errByKey["ResolveThread:THREAD_1"] = errors.New("boom")

		app, stdout, _ := newTestApp(fake)
		code := app.run([]string{"resolve", "99"})
		if code != 1 {
			t.Fatalf("expected runtime exit code 1, got %d", code)
		}
		if !strings.Contains(stdout.String(), "code: internal") {
			t.Fatalf("expected internal payload, got: %s", stdout.String())
		}
	})
}

func TestReplyResolvesThreadByDefault(t *testing.T) {
	fake := newFakeGitHub()
	fake.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}
	fake.threadByC["octo/repo#12@99"] = "THREAD_1"

	tmpDir := t.TempDir()
	msgFile := tmpDir + "/reply.md"
	if err := os.WriteFile(msgFile, []byte("thanks"), 0o644); err != nil {
		t.Fatalf("write message file: %v", err)
	}

	app, stdout, _ := newTestApp(fake)
	code := app.run([]string{"reply", "99", "--message-file", msgFile})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stdout.String())
	}
	if len(fake.replies) != 1 {
		t.Fatalf("expected one reply call, got %d", len(fake.replies))
	}
	if len(fake.resolved) != 1 || fake.resolved[0] != "THREAD_1" {
		t.Fatalf("expected resolved thread THREAD_1, got %#v", fake.resolved)
	}
	out := stdout.String()
	if !strings.Contains(out, "action: reply") {
		t.Fatalf("expected reply action, got: %s", out)
	}
	if !strings.Contains(out, "threadId: THREAD_1") {
		t.Fatalf("expected threadId in output, got: %s", out)
	}
}

func TestReplyResolveFailsAfterReply(t *testing.T) {
	fake := newFakeGitHub()
	fake.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}
	fake.threadByC["octo/repo#12@99"] = "THREAD_1"
	fake.errByKey["ResolveThread:THREAD_1"] = errors.New("boom")

	tmpDir := t.TempDir()
	msgFile := tmpDir + "/reply.md"
	if err := os.WriteFile(msgFile, []byte("thanks"), 0o644); err != nil {
		t.Fatalf("write message file: %v", err)
	}

	app, stdout, _ := newTestApp(fake)
	code := app.run([]string{"reply", "99", "--message-file", msgFile})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	// Reply should have been posted before resolve failed.
	if len(fake.replies) != 1 {
		t.Fatalf("expected one reply call, got %d", len(fake.replies))
	}
	out := stdout.String()
	if !strings.Contains(out, "createdCommentId: 1099") {
		t.Fatalf("expected createdCommentId in error details, got: %s", out)
	}
}

func TestReplyNoResolveOmitsThreadID(t *testing.T) {
	fake := newFakeGitHub()
	fake.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}

	tmpDir := t.TempDir()
	msgFile := tmpDir + "/reply.md"
	if err := os.WriteFile(msgFile, []byte("thanks"), 0o644); err != nil {
		t.Fatalf("write message file: %v", err)
	}

	app, stdout, _ := newTestApp(fake)
	code := app.run([]string{"reply", "99", "--message-file", msgFile, "--no-resolve"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stdout.String())
	}
	if strings.Contains(stdout.String(), "threadId") {
		t.Fatalf("expected no threadId with --no-resolve, got: %s", stdout.String())
	}
}

func TestReplyErrorPathClassification(t *testing.T) {
	fake := newFakeGitHub()
	fake.comments[keyComment("octo", "repo", 99)] = ghComment{ID: 99, PRNumber: 12}
	fake.errByKey["CreateReply:octo/repo#12@99"] = errors.New("boom")

	tmpDir := t.TempDir()
	msgFile := tmpDir + "/body.txt"
	if err := os.WriteFile(msgFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	app, stdout, _ := newTestApp(fake)
	code := app.run([]string{"reply", "99", "--message-file", msgFile})
	if code != 1 {
		t.Fatalf("expected runtime exit code 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "code: internal") {
		t.Fatalf("expected internal payload, got: %s", stdout.String())
	}
}
