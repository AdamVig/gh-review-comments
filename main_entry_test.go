package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestMainInvokesRunMainAndExit(t *testing.T) {
	oldRunMainForOS := runMainForOS
	oldExitMain := exitMain
	oldArgs := os.Args
	t.Cleanup(func() {
		runMainForOS = oldRunMainForOS
		exitMain = oldExitMain
		os.Args = oldArgs
	})

	os.Args = []string{"gh-review-comments", "list", "--help"}
	var (
		gotArgs     []string
		gotExitCode int
	)
	runMainForOS = func(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
		gotArgs = append([]string{}, args...)
		return 7
	}
	exitMain = func(code int) {
		gotExitCode = code
	}

	main()
	if !strings.EqualFold(strings.Join(gotArgs, " "), "list --help") {
		t.Fatalf("unexpected args passed to runMainForOS: %#v", gotArgs)
	}
	if gotExitCode != 7 {
		t.Fatalf("expected exit code 7 from stubbed runMainForOS, got %d", gotExitCode)
	}
}
