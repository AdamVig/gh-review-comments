package main

import "testing"

func TestParseCurrentPRFromJSONRejectsTrailingData(t *testing.T) {
	_, _, err := parseCurrentPRFromJSON(`{"number":7,"headRepository":{"name":"repo"},"headRepositoryOwner":{"login":"octo"}}{"extra":true}`)
	if err == nil {
		t.Fatalf("expected trailing JSON data error")
	}
}

func TestParseCurrentPRFromJSONValidPayload(t *testing.T) {
	number, repo, err := parseCurrentPRFromJSON(`{"number":7,"headRepository":{"name":"repo"},"headRepositoryOwner":{"login":"octo"}}`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if number != 7 || repo.Owner != "octo" || repo.Name != "repo" {
		t.Fatalf("unexpected parsed values: number=%d repo=%#v", number, repo)
	}
}
