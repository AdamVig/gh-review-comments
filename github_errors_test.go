package main

import (
	"errors"
	"net/http"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

func TestClassifyAPIError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{name: "not found sentinel", err: errNotFound, code: "notfound"},
		{name: "http unauthorized", err: &api.HTTPError{StatusCode: http.StatusUnauthorized}, code: "auth"},
		{name: "http forbidden", err: &api.HTTPError{StatusCode: http.StatusForbidden}, code: "forbidden"},
		{name: "http not found", err: &api.HTTPError{StatusCode: http.StatusNotFound}, code: "notfound"},
		{name: "http generic", err: &api.HTTPError{StatusCode: http.StatusBadGateway}, code: "api"},
		{name: "gql forbidden", err: &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Type: "FORBIDDEN", Message: "denied"}}}, code: "forbidden"},
		{name: "gql not found", err: &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Type: "NOT_FOUND", Message: "missing"}}}, code: "notfound"},
		{name: "gql generic", err: &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Type: "SOMETHING_ELSE", Message: "boom"}}}, code: "api"},
		{name: "auth string fallback", err: errors.New("authentication failed"), code: "auth"},
		{name: "internal fallback", err: errors.New("some other error"), code: "internal"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyAPIError(tc.err, map[string]any{"endpoint": "test"})
			if got.Code != tc.code {
				t.Fatalf("expected code %q, got %q (%#v)", tc.code, got.Code, got)
			}
			if got.Exit != exitError {
				t.Fatalf("expected exit code %d, got %d", exitError, got.Exit)
			}
		})
	}
}
