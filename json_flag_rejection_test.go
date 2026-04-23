package main

import (
	"strings"
	"testing"
)

func TestUnsupportedJSONFlagAcrossCommands(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"list", "--json"}},
		{name: "reply", args: []string{"reply", "99", "--message-file", "dummy.txt", "--json"}},
		{name: "resolve", args: []string{"resolve", "99", "--json"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeGitHub()
			app, stdout, _ := newTestApp(fake)

			code := app.run(tc.args)
			if code != exitUsage {
				t.Fatalf("expected usage exit code %d, got %d", exitUsage, code)
			}

			out := stdout.String()
			for _, want := range []string{
				"ok: false",
				"code: usage",
				"--json is not supported by gh review-comments",
				"hint: remove --json; this extension always outputs TOON",
				`unsupportedFlag: "--json"`,
			} {
				if !strings.Contains(out, want) {
					t.Fatalf("expected output to contain %q, got: %s", want, out)
				}
			}

			if len(fake.replies) != 0 {
				t.Fatalf("expected no reply calls on parse error, got %#v", fake.replies)
			}
			if len(fake.resolved) != 0 {
				t.Fatalf("expected no resolve calls on parse error, got %#v", fake.resolved)
			}
			if fake.listPRDataCalls != 0 || fake.getPRCalls != 0 || fake.getThreadsCalls != 0 || fake.getReviewBodyCalls != 0 {
				t.Fatalf("expected no list API calls on parse error, got ListPRData=%d GetPR=%d GetReviewThreads=%d GetReviewBodies=%d",
					fake.listPRDataCalls, fake.getPRCalls, fake.getThreadsCalls, fake.getReviewBodyCalls)
			}
		})
	}
}
