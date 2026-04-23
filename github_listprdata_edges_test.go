package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/require"
	gock "gopkg.in/h2non/gock.v1"
)

func TestListPRDataSkipsNonPositiveNumbers(t *testing.T) {
	client := &gitHubClient{}
	data, perErr, err := client.ListPRData("octo", "repo", []int{0, -1, 0})
	require.NoError(t, err)
	require.Empty(t, data)
	require.Empty(t, perErr)
}

func TestListPRDataMissingRepositoryInResponse(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/graphql").
		Reply(200).
		JSON(map[string]any{"data": map[string]any{}})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	_, _, err = client.ListPRData("octo", "repo", []int{1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing repository in list batch response")
}

func TestListPRDataPerPRErrorsAndNumberFallback(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/graphql").
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"repository": map[string]any{
				"pr0": nil,
				"pr1": "{",
				"pr2": map[string]any{
					"number": 0,
					"title":  "Fallback",
					"url":    "https://github.com/octo/repo/pull/12",
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

	data, perErr, err := client.ListPRData("octo", "repo", []int{10, 11, 12})
	require.NoError(t, err)
	require.ErrorIs(t, perErr[10], errNotFound)
	require.Error(t, perErr[11])
	require.Contains(t, perErr[11].Error(), "failed to parse list batch response")
	require.Equal(t, 12, data[12].PR.Number)
}

func TestListPRDataChunksLargerInputs(t *testing.T) {
	t.Cleanup(gock.Off)

	firstChunkRepo := map[string]any{}
	for i := range 10 {
		n := i + 1
		firstChunkRepo[fmt.Sprintf("pr%d", i)] = map[string]any{
			"number": n,
			"title":  "PR",
			"url":    fmt.Sprintf("https://github.com/octo/repo/pull/%d", n),
			"reviewThreads": map[string]any{
				"nodes":    []any{},
				"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
			},
			"reviews": map[string]any{
				"nodes":    []any{},
				"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
			},
		}
	}

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			s := string(body)
			return strings.Contains(s, "$n9:Int!") && strings.Contains(s, "pr9: pullRequest(number:$n9)"), nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{"repository": firstChunkRepo}})

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			s := string(body)
			return strings.Contains(s, "$n0:Int!") && !strings.Contains(s, "$n1:Int!"), nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"repository": map[string]any{
				"pr0": map[string]any{
					"number": 11,
					"title":  "PR",
					"url":    "https://github.com/octo/repo/pull/11",
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

	data, perErr, err := client.ListPRData("octo", "repo", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	require.NoError(t, err)
	require.Empty(t, perErr)
	require.Len(t, data, 11)
	require.True(t, gock.IsDone())
}

func TestListPRDataFallsBackToPaginatedEndpoints(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			return strings.Contains(string(body), "pr0: pullRequest(number:$n0)"), nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"repository": map[string]any{
				"pr0": map[string]any{
					"number": 7,
					"title":  "Fallback",
					"url":    "https://github.com/octo/repo/pull/7",
					"reviewThreads": map[string]any{
						"nodes":    []any{},
						"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "abc"},
					},
					"reviews": map[string]any{
						"nodes":    []any{},
						"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "abc"},
					},
				},
			},
		}})

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			return strings.Contains(string(body), "reviewThreads(first:100, after:$after)"), nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"repository": map[string]any{
				"pullRequest": map[string]any{
					"reviewThreads": map[string]any{
						"nodes": []map[string]any{
							{
								"id":         "THREAD_1",
								"path":       "a.go",
								"line":       1,
								"diffSide":   "RIGHT",
								"isOutdated": false,
								"isResolved": false,
								"comments": map[string]any{
									"nodes":    []map[string]any{{"databaseId": 99, "body": "x", "createdAt": "2025-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}}},
									"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
								},
							},
						},
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					},
				},
			},
		}})

	gock.New("https://api.github.com").
		Get("/repos/octo/repo/pulls/7/reviews").
		MatchParam("per_page", "100").
		Reply(200).
		JSON([]map[string]any{
			{"id": 1, "submitted_at": "2025-01-01T00:00:00Z", "user": map[string]any{"login": "bot"}, "body": "review"},
		})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	data, perErr, err := client.ListPRData("octo", "repo", []int{7})
	require.NoError(t, err)
	require.Empty(t, perErr)
	require.Len(t, data[7].Threads, 1)
	require.Len(t, data[7].Reviews, 1)
	require.True(t, gock.IsDone())
}

func TestConvertBatchThreadsFetchesExtraCommentPages(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			return strings.Contains(string(body), "node(id:$threadId)"), nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"node": map[string]any{
				"comments": map[string]any{
					"nodes": []map[string]any{
						{"databaseId": 100, "body": "extra", "createdAt": "2025-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}},
					},
					"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
				},
			},
		}})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	id := int64(99)
	threads, err := client.convertBatchThreads("octo", "repo", 7, listBatchThreadConn{
		Nodes: []listBatchThreadNode{
			{
				ID:   "THREAD_1",
				Path: "a.go",
				Comments: listBatchCommentConn{
					Nodes: []listBatchCommentNode{{DatabaseID: &id, Body: "first", CreatedAt: "2025-01-01T00:00:00Z"}},
					PageInfo: listBatchPageInfo{
						HasNextPage: true,
						EndCursor:   nil,
					},
				},
			},
		},
		PageInfo: listBatchPageInfo{HasNextPage: false},
	})
	require.NoError(t, err)
	require.Len(t, threads, 1)
	require.Len(t, threads[0].Comments, 2)
	require.True(t, gock.IsDone())
}

func TestClientFactoryErrorPaths(t *testing.T) {
	client := &gitHubClient{}
	_, err := client.restClient()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing rest client factory")

	client = &gitHubClient{restNew: func() (*api.RESTClient, error) { return nil, errors.New("rest init failed") }}
	_, err = client.restClient()
	require.Error(t, err)
	require.Contains(t, err.Error(), "rest init failed")

	client = &gitHubClient{}
	_, err = client.gqlClient()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing graphql client factory")

	client = &gitHubClient{graphqlNew: func() (*api.GraphQLClient, error) { return nil, errors.New("gql init failed") }}
	_, err = client.gqlClient()
	require.Error(t, err)
	require.Contains(t, err.Error(), "gql init failed")
}

func TestGetPRFallsBackToRequestedNumber(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Get("/repos/octo/repo/pulls/12").
		Reply(200).
		JSON(map[string]any{
			"number":   0,
			"title":    "PR title",
			"html_url": "https://github.com/octo/repo/pull/12",
		})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	pr, err := client.GetPR("octo", "repo", 12)
	require.NoError(t, err)
	require.Equal(t, 12, pr.Number)
}

func TestListPRDataGQLClientFactoryFailure(t *testing.T) {
	client := &gitHubClient{
		graphqlNew: func() (*api.GraphQLClient, error) {
			return nil, errors.New("no gql")
		},
	}
	_, _, err := client.ListPRData("octo", "repo", []int{1})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no gql")
}
