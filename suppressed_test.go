package main

import "testing"

func TestParseSuppressedComments_MultipleBlocksAndCodeFence(t *testing.T) {
	body := "<details>\n<summary>Comments suppressed due to low confidence (2)</summary>\n\n**a.go:10**\n* First explanation\n\n```go\nfmt.Println(\"x\")\n```\n\n**b.go:20**\n* Second explanation\n</details>\n\n<details>\n<summary>Comments suppressed due to low confidence (1)</summary>\n\n**c.go:30**\n* Third explanation\n</details>"

	got := parseSuppressedComments(body)
	if len(got) != 3 {
		t.Fatalf("expected 3 suppressed items, got %d", len(got))
	}
	if got[0].Path != "a.go" || got[0].Line != 10 || got[0].Index != 1 {
		t.Fatalf("first item mismatch: %#v", got[0])
	}
	if got[0].Body != "First explanation\n```go\nfmt.Println(\"x\")\n```" {
		t.Fatalf("first body mismatch: %q", got[0].Body)
	}
	if got[1].Path != "b.go" || got[1].Line != 20 || got[1].Index != 2 {
		t.Fatalf("second item mismatch: %#v", got[1])
	}
	if got[2].Path != "c.go" || got[2].Line != 30 || got[2].Index != 3 {
		t.Fatalf("third item mismatch: %#v", got[2])
	}
}

func TestParseSuppressedComments_MissingPartsAreSkipped(t *testing.T) {
	body := `<details>
<summary>Comments suppressed due to low confidence (3)</summary>

**a.go:10**
No bullet here

**badheader**
* ignored

**b.go:20**
* valid
</details>`

	got := parseSuppressedComments(body)
	if len(got) != 1 {
		t.Fatalf("expected 1 parsed item, got %d", len(got))
	}
	if got[0].Path != "b.go" || got[0].Line != 20 || got[0].Body != "valid" {
		t.Fatalf("unexpected parsed item: %#v", got[0])
	}
}

func TestParseSuppressedComments_WhitespaceTolerance(t *testing.T) {
	body := `<details>
<summary> Comments suppressed due to low confidence (1) </summary>

**pkg/file.go:7**

*   reason with spaces   

</details>`

	got := parseSuppressedComments(body)
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0].Path != "pkg/file.go" || got[0].Line != 7 || got[0].Body != "reason with spaces" {
		t.Fatalf("unexpected item: %#v", got[0])
	}
}
