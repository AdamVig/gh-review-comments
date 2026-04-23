# gh-review-comments

Minimal `gh` extension for listing actionable pull request review threads,
replying to a review comment, and resolving the containing thread.

It is built around a stable, machine-friendly output format so it works well in
shell scripts, editor tooling, and other automation without adding an
interactive layer on top of `gh`.

## Install

Requirements:

- GitHub CLI (`gh`) installed and authenticated
- `git-spice` installed if you want to use `list-stack`

Install from GitHub:

```bash
gh extension install AdamVig/gh-review-comments
```

Install from a local checkout while developing:

```bash
gh extension install .
```

Contributor notes live in [`ARCHITECTURE.md`](ARCHITECTURE.md) and
[`TESTING.md`](TESTING.md).

## Command contracts

- `gh review-comments list [--pr <N|URL> ...] [--repo OWNER/REPO] [--author <login> ...] [--max-body N]`
- `gh review-comments list-stack [--repo OWNER/REPO] [--author <login> ...] [--max-body N]`
- `gh review-comments reply <comment-id> --message-file <path> [--repo OWNER/REPO]`
- `gh review-comments resolve <comment-id> [--repo OWNER/REPO]`

`resolve` and `reply` accept a comment-id (not a thread ID) because the GitHub
API has no stable thread ID in REST responses. Pass any comment-id within the
target thread; the entire thread is resolved or receives the reply.

`list` suppressed items are low-confidence review suggestions that the reviewing
tool (e.g. Copilot) embedded inside HTML `<details>` blocks in a review body
instead of posting as standalone threads. They are informational only — not
unresolved threads — and cannot be resolved or replied to. Only suppressed
comments from the latest review authored by one of the active `--author` filters
are included (omitting `--author` considers all authors).
Use `list-stack` to discover and list threads across all PRs in the current
`git-spice` stack.
If passing bracketed author logins in zsh, quote them (for example `'name[bot]'`).
When multiple `--pr` values are provided, they must all resolve to the same
repository.
Reply body input must be non-empty after trimming whitespace.
Write the reply body to a temporary file and pass it via `--message-file`.

## Output contract (TOON)

- Non-help invocations print exactly one TOON document to stdout.
  TOON (Token-Oriented Object Notation) is a compact, human-readable
  serialization format that is easy for both humans and automation to inspect.
  See
  https://github.com/toon-format/toon-go for the spec and parser libraries.
- Help invocations print human-readable help (not TOON).
- `--json` is intentionally unsupported; consumers should parse TOON.

## Failure model (codes + exit codes)

Stable failure codes: `usage`, `repo`, `auth`, `api`, `notfound`,
`forbidden`, `parse`, `internal`.

Exit codes:

- `0` success (including partial-success list when at least one PR succeeds)
- `1` runtime/API/auth/internal errors
- `2` usage/argument errors

## Examples

```bash
gh review-comments list
gh review-comments list-stack
gh review-comments list --pr 123 --max-body 300
gh review-comments reply 987654 --message-file /tmp/reply.md
gh review-comments resolve 987654
```

## Dev commands (make targets)

- `make test`
- `make test-race`
- `make vet`
- `make fmt`
- `make fix`
- `make check`
- `make build`

## Limitations / Non-goals

- No daemon mode, caching layer, config files, or interactive prompts.
- No generic GraphQL passthrough.
- Only list/reply/resolve workflows are implemented.
