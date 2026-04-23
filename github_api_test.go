package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/require"
	gock "gopkg.in/h2non/gock.v1"
)

func TestReplyRESTEndpointsAndPayload(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Get("/repos/octo/repo/pulls/comments/99").
		Reply(200).
		JSON(map[string]any{"id": 99, "pull_request_url": "https://api.github.com/repos/octo/repo/pulls/12"})

	gock.New("https://api.github.com").
		Post("/repos/octo/repo/pulls/12/comments").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			b, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				return false, err
			}
			return got["body"] == "thanks" && int(got["in_reply_to"].(float64)) == 99, nil
		}).
		Reply(201).
		JSON(map[string]any{"id": 1234})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	comment, err := client.GetComment("octo", "repo", 99)
	require.NoError(t, err)
	require.Equal(t, 12, comment.PRNumber)

	created, err := client.CreateReply("octo", "repo", comment.PRNumber, 99, "thanks")
	require.NoError(t, err)
	require.Equal(t, int64(1234), created)
	require.True(t, gock.IsDone())
}

func TestResolveGraphQLEndpointsAndPayload(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			b, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				return false, err
			}
			q, _ := got["query"].(string)
			if !strings.Contains(q, "reviewThreads(first:100") {
				return false, nil
			}
			vars, _ := got["variables"].(map[string]any)
			return vars["owner"] == "octo" && vars["repo"] == "repo" && int(vars["number"].(float64)) == 12, nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"repository": map[string]any{
				"pullRequest": map[string]any{
					"reviewThreads": map[string]any{
						"nodes": []map[string]any{{
							"id":         "THREAD_1",
							"path":       "a.go",
							"line":       1,
							"diffSide":   "RIGHT",
							"isOutdated": false,
							"isResolved": false,
							"comments": map[string]any{
								"nodes":    []map[string]any{{"databaseId": 99, "body": "x", "createdAt": "2024-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}}},
								"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
							},
						}},
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					},
				},
			},
		}})

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			b, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				return false, err
			}
			q, _ := got["query"].(string)
			if !strings.Contains(q, "resolveReviewThread") {
				return false, nil
			}
			vars, _ := got["variables"].(map[string]any)
			return vars["threadId"] == "THREAD_1", nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"resolveReviewThread": map[string]any{"thread": map[string]any{"id": "THREAD_1"}},
		}})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	threadID, err := client.FindThreadIDByComment("octo", "repo", 12, 99)
	require.NoError(t, err)
	require.Equal(t, "THREAD_1", threadID)

	err = client.ResolveThread(threadID)
	require.NoError(t, err)
	require.True(t, gock.IsDone())
}

func TestListPRDataBatchesTwoPullRequestsInOneGraphQLCall(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			b, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				return false, err
			}
			q, _ := got["query"].(string)
			if !strings.Contains(q, "pr0: pullRequest(number:$n0)") || !strings.Contains(q, "pr1: pullRequest(number:$n1)") {
				return false, nil
			}
			vars, _ := got["variables"].(map[string]any)
			return vars["owner"] == "octo" && vars["repo"] == "repo" && int(vars["n0"].(float64)) == 1 && int(vars["n1"].(float64)) == 2, nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"repository": map[string]any{
				"pr0": map[string]any{
					"number": 1,
					"title":  "PR One",
					"url":    "https://github.com/octo/repo/pull/1",
					"reviewThreads": map[string]any{
						"nodes":    []any{},
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					},
					"reviews": map[string]any{
						"nodes":    []any{},
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					},
				},
				"pr1": map[string]any{
					"number": 2,
					"title":  "PR Two",
					"url":    "https://github.com/octo/repo/pull/2",
					"reviewThreads": map[string]any{
						"nodes":    []any{},
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					},
					"reviews": map[string]any{
						"nodes":    []any{},
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					},
				},
			},
		}})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	data, perErr, err := client.ListPRData("octo", "repo", []int{1, 2})
	require.NoError(t, err)
	require.Empty(t, perErr)
	require.Len(t, data, 2)
	require.Equal(t, "PR One", data[1].PR.Title)
	require.Equal(t, "PR Two", data[2].PR.Title)
	require.True(t, gock.IsDone())
}

func TestGetPRRESTEndpoint(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Get("/repos/octo/repo/pulls/12").
		Reply(200).
		JSON(map[string]any{
			"number":   12,
			"title":    "PR title",
			"html_url": "https://github.com/octo/repo/pull/12",
		})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	pr, err := client.GetPR("octo", "repo", 12)
	require.NoError(t, err)
	require.Equal(t, 12, pr.Number)
	require.Equal(t, "PR title", pr.Title)
	require.Equal(t, "https://github.com/octo/repo/pull/12", pr.URL)
	require.True(t, gock.IsDone())
}

func TestGetReviewBodiesPaginatesAndSkipsEmptyBodies(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Get("/repos/octo/repo/pulls/12/reviews").
		MatchParam("per_page", "100").
		Reply(200).
		SetHeader("Link", `<https://api.github.com/repos/octo/repo/pulls/12/reviews?per_page=100&page=2>; rel="next"`).
		JSON([]map[string]any{
			{"id": 1, "submitted_at": "2025-01-01T00:00:00Z", "user": map[string]any{"login": "bot"}, "body": " "},
			{"id": 2, "submitted_at": "2025-01-02T00:00:00Z", "user": map[string]any{"login": "bot"}, "body": "keep-1"},
		})

	gock.New("https://api.github.com").
		Get("/repos/octo/repo/pulls/12/reviews").
		MatchParam("per_page", "100").
		MatchParam("page", "2").
		Reply(200).
		JSON([]map[string]any{
			{"id": 3, "submitted_at": "2025-01-03T00:00:00Z", "user": map[string]any{"login": "bot"}, "body": "keep-2"},
		})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	reviews, err := client.GetReviewBodies("octo", "repo", 12)
	require.NoError(t, err)
	require.Len(t, reviews, 2)
	require.Equal(t, int64(2), reviews[0].ReviewID)
	require.Equal(t, "keep-1", reviews[0].Body)
	require.Equal(t, int64(3), reviews[1].ReviewID)
	require.Equal(t, "keep-2", reviews[1].Body)
	require.True(t, gock.IsDone())
}

func TestGetThreadCommentsPageGraphQLEndpoint(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			b, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				return false, err
			}
			q, _ := got["query"].(string)
			if !strings.Contains(q, "node(id:$threadId)") {
				return false, nil
			}
			vars, _ := got["variables"].(map[string]any)
			return vars["threadId"] == "THREAD_1", nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"node": map[string]any{
				"comments": map[string]any{
					"nodes": []map[string]any{
						{"databaseId": 101, "body": "a", "createdAt": "2025-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}},
						{"databaseId": 102, "body": "b", "createdAt": "2025-01-01T00:00:01Z", "author": map[string]any{"login": "bot"}},
					},
					"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
				},
			},
		}})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	comments, err := client.getThreadCommentsPage("THREAD_1", nil)
	require.NoError(t, err)
	require.Len(t, comments, 2)
	require.Equal(t, int64(101), comments[0].ID)
	require.Equal(t, int64(102), comments[1].ID)
	require.True(t, gock.IsDone())
}

func TestLazyClientFactories(t *testing.T) {
	restCalls := 0
	gqlCalls := 0
	client := &gitHubClient{
		restNew: func() (*api.RESTClient, error) {
			restCalls++
			return &api.RESTClient{}, nil
		},
		graphqlNew: func() (*api.GraphQLClient, error) {
			gqlCalls++
			return &api.GraphQLClient{}, nil
		},
	}

	_, err := client.restClient()
	require.NoError(t, err)
	_, err = client.restClient()
	require.NoError(t, err)
	_, err = client.gqlClient()
	require.NoError(t, err)
	_, err = client.gqlClient()
	require.NoError(t, err)

	require.Equal(t, 1, restCalls)
	require.Equal(t, 1, gqlCalls)
}
