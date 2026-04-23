package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	gh "github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/repository"
	toon "github.com/toon-format/toon-go"
)

type githubAPI interface {
	ListPRData(owner, repo string, numbers []int) (map[int]ghListPRData, map[int]error, error)
	GetPR(owner, repo string, number int) (ghPR, error)
	GetReviewThreads(owner, repo string, number int) ([]ghThread, error)
	GetReviewBodies(owner, repo string, number int) ([]ghReviewBody, error)
	GetComment(owner, repo string, commentID int64) (ghComment, error)
	CreateReply(owner, repo string, prNumber int, inReplyTo int64, body string) (int64, error)
	FindThreadIDByComment(owner, repo string, prNumber int, commentID int64) (string, error)
	ResolveThread(threadID string) error
}

type app struct {
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	ghapi       githubAPI
	repoCurrent func() (repository.Repository, error)
	repoParse   func(string) (repository.Repository, error)
	ghExec      func(args ...string) (string, string, error)
	gitSpiceLog func() (string, string, error)
}

func newDefaultApp(stdin io.Reader, stdout, stderr io.Writer) *app {
	return &app{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		ghapi:  newGitHubClient(),
		repoCurrent: func() (repository.Repository, error) {
			return repository.Current()
		},
		repoParse: repository.Parse,
		ghExec: func(args ...string) (string, string, error) {
			out, errOut, err := gh.Exec(args...)
			return out.String(), errOut.String(), err
		},
		gitSpiceLog: runGitSpiceLog,
	}
}

var runMainForOS = runMain
var exitMain = os.Exit

func main() {
	exitMain(runMainForOS(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	app := newDefaultApp(stdin, stdout, stderr)
	return app.run(args)
}

func (a *app) run(args []string) int {
	payload, isHelp, exitCode := a.dispatch(args)
	if isHelp {
		_, _ = io.WriteString(a.stdout, payload.(string))
		return exitCode
	}
	if err := emitTOON(a.stdout, payload); err != nil {
		fallback := failurePayload{
			OK:      false,
			Code:    "internal",
			Message: "failed to encode output",
			Hint:    "retry with GH_DEBUG=api and inspect stderr",
			Details: map[string]any{"error": err.Error()},
		}
		_ = emitTOON(a.stdout, fallback)
		fmt.Fprintf(a.stderr, "encode error: %v\n", err)
		return exitError
	}
	return exitCode
}

func emitTOON(w io.Writer, v any) error {
	b, err := toon.Marshal(v, toon.WithLengthMarkers(true))
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func (a *app) dispatch(args []string) (any, bool, int) {
	if len(args) == 0 || isHelpArg(args[0]) {
		return rootHelp(), true, exitSuccess
	}

	switch args[0] {
	case "list":
		if len(args) > 1 && isHelpArg(args[1]) {
			return listHelp(), true, exitSuccess
		}
		payload, appErr := a.runList(args[1:])
		return payloadOrError(payload, appErr)
	case "list-stack":
		if len(args) > 1 && isHelpArg(args[1]) {
			return listStackHelp(), true, exitSuccess
		}
		payload, appErr := a.runListStack(args[1:])
		return payloadOrError(payload, appErr)
	case "reply":
		if len(args) > 1 && isHelpArg(args[1]) {
			return replyHelp(), true, exitSuccess
		}
		payload, appErr := a.runReply(args[1:])
		return payloadOrError(payload, appErr)
	case "resolve":
		if len(args) > 1 && isHelpArg(args[1]) {
			return resolveHelp(), true, exitSuccess
		}
		payload, appErr := a.runResolve(args[1:])
		return payloadOrError(payload, appErr)
	case "--version", "version":
		return "gh-review-comments\n", true, exitSuccess
	default:
		appErr := newUsageError(
			"unknown command",
			"run `gh review-comments --help`",
			map[string]any{"command": args[0]},
		)
		return payloadOrError(nil, appErr)
	}
}

func payloadOrError(payload any, appErr *appError) (any, bool, int) {
	if appErr == nil {
		return payload, false, exitSuccess
	}
	details := appErr.Details
	if details == nil {
		details = map[string]any{}
	}
	return failurePayload{
		OK:      false,
		Code:    appErr.Code,
		Message: appErr.Message,
		Hint:    appErr.Hint,
		Details: details,
	}, false, appErr.Exit
}

func isHelpArg(v string) bool {
	return v == "--help" || v == "-h" || v == "help"
}

func newFlagSet(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	return f
}

func parseFlagError(err error, hint string) *appError {
	if err == nil {
		return nil
	}
	if isUnsupportedJSONFlagError(err) {
		return newUsageError(
			"--json is not supported by gh review-comments",
			"remove --json; this extension always outputs TOON",
			map[string]any{"unsupportedFlag": "--json"},
		)
	}
	return newUsageError("invalid arguments", hint, map[string]any{"error": err.Error()})
}

func isUnsupportedJSONFlagError(err error) bool {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "flag provided but not defined: -json") ||
		strings.Contains(msg, "flag provided but not defined: --json")
}

func readBodyFromFile(file string) (string, *appError) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", &appError{Code: "usage", Message: "failed to read message file", Hint: "pass a valid --message-file path", Details: map[string]any{"path": file, "error": err.Error()}, Exit: exitUsage}
	}
	body := string(b)
	if strings.TrimSpace(body) == "" {
		return "", newUsageError(
			"reply body must not be empty",
			"pass non-empty text via --message-file",
			map[string]any{"path": file},
		)
	}
	return body, nil
}

func parseRepo(repoArg string, repoParse func(string) (repository.Repository, error)) (repository.Repository, *appError) {
	repo, err := repoParse(repoArg)
	if err != nil {
		return repository.Repository{}, &appError{
			Code:    "repo",
			Message: "invalid repository",
			Hint:    "pass --repo as OWNER/REPO",
			Details: map[string]any{"repo": repoArg, "error": err.Error()},
			Exit:    exitUsage,
		}
	}
	return repo, nil
}

func inferRepo(current func() (repository.Repository, error)) (repository.Repository, *appError) {
	repo, err := current()
	if err != nil {
		return repository.Repository{}, &appError{
			Code:    "repo",
			Message: "could not infer repository",
			Hint:    "run from a git checkout or pass --repo OWNER/REPO",
			Details: map[string]any{"error": err.Error()},
			Exit:    exitError,
		}
	}
	return repo, nil
}

func repoString(r repository.Repository) string {
	if r.Owner == "" || r.Name == "" {
		return ""
	}
	return r.Owner + "/" + r.Name
}

func parseCurrentPRFromJSON(raw string) (int, repository.Repository, error) {
	type prView struct {
		Number         int `json:"number"`
		HeadRepository struct {
			Name string `json:"name"`
		} `json:"headRepository"`
		HeadRepositoryOwner struct {
			Login string `json:"login"`
		} `json:"headRepositoryOwner"`
	}
	var out prView
	if err := decodeJSON(raw, &out); err != nil {
		return 0, repository.Repository{}, err
	}
	if out.Number <= 0 || out.HeadRepositoryOwner.Login == "" || out.HeadRepository.Name == "" {
		return 0, repository.Repository{}, fmt.Errorf("missing number/repository in gh pr view output")
	}
	return out.Number, repository.Repository{Owner: out.HeadRepositoryOwner.Login, Name: out.HeadRepository.Name, Host: "github.com"}, nil
}

func decodeJSON(raw string, target any) error {
	dec := json.NewDecoder(strings.NewReader(raw))
	if err := dec.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON data")
	}
	return nil
}
