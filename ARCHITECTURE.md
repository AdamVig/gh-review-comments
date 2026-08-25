# Architecture

## Invariants

- Non-help commands emit exactly one TOON payload on stdout.
- Deterministic ordering for authors, PRs, threads, comments, reviews, and
  suppressed entries.
- Stable error payload shape and code taxonomy.
- Full pagination for all relevant GitHub collections.

## Where to edit for X

- CLI args/help/exit behavior: `main.go`
- `list` and `list-stack` behavior and ordering: `cmd_list.go`
- `reply` behavior: `cmd_reply.go`
- `resolve` behavior: `cmd_resolve.go`
- GitHub REST/GraphQL calls: `github.go`
- GitHub stack discovery/parsing: `stack.go`
- suppressed review parsing: `suppressed.go`
- TOON payload structs: `output_types.go`
- tests: `*_test.go`
