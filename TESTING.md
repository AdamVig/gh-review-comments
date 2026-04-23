# Testing

## Rationale

Tests focus on behavior contracts and deterministic output, not incidental
implementation details.

- Assert stdout purity and single-TOON output for non-help paths.
- Assert ordering guarantees for authors, PRs, threads, and comments.
- Use snapshot/golden output tests for list payload rendering.
- Validate suppressed parser robustness for malformed and mixed content.
- Validate git-spice parsing/fallback behavior.
- Validate reply/resolve endpoint wiring and request payloads.

## Principles

- Avoid overlapping tests that assert the same behavior in multiple places.
- Assert only fields relevant to the contract.
- Prefer high coverage but avoid brittle fixture coupling.
