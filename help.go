package main

func rootHelp() string {
	return `gh review-comments

Minimal gh extension for PR review comment workflows with stable,
machine-friendly output.

Commands:
  list        List unresolved review threads and suppressed comments
  list-stack  Like list, but discovers PRs from the git-spice stack
  reply       Reply to a PR review comment and resolve the thread
  resolve     Resolve the review thread containing comment-id

Notes:
  - Non-help commands output exactly one TOON (Token-Oriented Object Notation)
    document on stdout. TOON is a compact, human-readable serialization format.
    See https://github.com/toon-format/toon-go for the spec and parsers.
  - --json is intentionally unsupported; parse TOON output instead.
  - Skip preflight auth checks; only run gh auth status after code: auth.
  - comment-id is the numeric Pull Request Review Comment database ID:
    GET /repos/{owner}/{repo}/pulls/comments/{comment_id}
  - resolve and reply accept a comment-id, not a thread ID — the entire
    thread containing that comment is resolved. The GitHub API has no stable
    thread ID in REST responses, so any comment-id within a thread serves as
    the handle.

Examples:
  gh review-comments list
  gh review-comments list-stack
  gh review-comments list --pr 123 --max-body 300
  gh review-comments reply 987654 --message-file /tmp/reply.md
  gh review-comments reply 987654 --message-file /tmp/reply.md --no-resolve
  gh review-comments resolve 987654
`
}

func listHelp() string {
	return `gh review-comments list [--pr <N|URL> ...] [--repo OWNER/REPO] [--author <login> ...] [--max-body N]

List unresolved PR review threads matching authors and informational
suppressed comments from review bodies.

Suppressed comments are low-confidence review suggestions that the reviewing
tool (e.g. Copilot) embedded inside HTML <details> blocks in a review body
instead of posting as standalone review threads. They are informational only —
not unresolved threads — and cannot be resolved or replied to. Only suppressed
comments from the latest matching review per author are included.

Notes:
  - Non-help output is one TOON document.
  - --json is intentionally unsupported; parse TOON output instead.
  - Omitting --author returns comments from all authors.
  - To see only Copilot comments: --author copilot-pull-request-reviewer
  - --max-body N truncates body and title fields to at most N runes
    (Unicode characters). 0 removes bodies entirely.
  - In zsh, quote bracketed author values like 'name[bot]'.
  - When passing multiple --pr values, they must all target one repository.
  - For stack-wide listing, use list-stack instead.

Examples:
  gh review-comments list
  gh review-comments list --author copilot-pull-request-reviewer
  gh review-comments list --pr 123 --max-body 300
`
}

func listStackHelp() string {
	return `gh review-comments list-stack [--repo OWNER/REPO] [--author <login> ...] [--max-body N]

List unresolved review threads across all PRs in the current git-spice stack.
Equivalent to list but discovers PRs automatically via git-spice.

Notes:
  - Non-help output is one TOON document.
  - --json is intentionally unsupported; parse TOON output instead.
  - Requires git-spice to be installed and the branch to be tracked.
  - Omitting --author returns comments from all authors.
  - --max-body N truncates body and title fields to at most N runes
    (Unicode characters). 0 removes bodies entirely.

Examples:
  gh review-comments list-stack
  gh review-comments list-stack --author copilot-pull-request-reviewer
`
}

func replyHelp() string {
	return `gh review-comments reply <comment-id> --message-file <path> [--no-resolve] [--repo OWNER/REPO]

Reply to a PR review comment and resolve the thread.

comment-id meaning:
  Numeric Pull Request Review Comment database ID addressable at:
  GET /repos/{owner}/{repo}/pulls/comments/{comment_id}

Notes:
  - Non-help output is one TOON document.
  - --json is intentionally unsupported; parse TOON output instead.
  - --message-file is the only input mode. Write the reply body to a
    temporary file first. This avoids shell-interpolation issues with
    inline text and handles Markdown/code fences safely.
  - Reply body must be non-empty after trimming whitespace.
  - The thread is resolved by default after posting the reply. The
    output includes a threadId field when resolution succeeds.
  - --no-resolve skips resolution, leaving the thread open.
  - If resolution fails, the reply has already been posted — the error
    details include createdCommentId so you know what landed.

Examples:
  gh review-comments reply 987654 --message-file /tmp/reply.md
  gh review-comments reply 987654 --message-file /tmp/reply.md --no-resolve
  gh review-comments reply 987654 --message-file /tmp/reply.md --repo octo/repo
`
}

func resolveHelp() string {
	return `gh review-comments resolve <comment-id> [--repo OWNER/REPO]

Resolve the review thread containing comment-id. You are resolving the
entire thread — pass any comment-id within it.

comment-id meaning:
  Numeric Pull Request Review Comment database ID addressable at:
  GET /repos/{owner}/{repo}/pulls/comments/{comment_id}

Why comment-id instead of thread-id:
  The GitHub API does not expose a stable thread ID in REST responses.
  Every thread contains at least one comment, so a comment-id uniquely
  identifies the thread. Use anchorCommentId from the list output.

Notes:
  - Non-help output is one TOON document.
  - --json is intentionally unsupported; parse TOON output instead.

Examples:
  gh review-comments resolve 987654
  gh review-comments resolve 987654 --repo octo/repo
`
}
