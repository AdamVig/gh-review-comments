package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/require"
	gock "gopkg.in/h2non/gock.v1"
)

func TestGetCommentInvalidPullRequestURL(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Get("/repos/octo/repo/pulls/comments/99").
		Reply(200).
		JSON(map[string]any{"id": 99, "pull_request_url": "https://api.github.com/repos/octo/repo/issues/12"})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	_, err = client.GetComment("octo", "repo", 99)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid pull_request_url")
}

func TestCreateReplyMissingCreatedID(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/repos/octo/repo/pulls/12/comments").
		Reply(201).
		JSON(map[string]any{"id": 0})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	_, err = client.CreateReply("octo", "repo", 12, 99, "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing created comment id")
}

func TestResolveThreadMissingThreadID(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Post("/graphql").
		AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return false, err
			}
			return strings.Contains(string(body), "resolveReviewThread"), nil
		}).
		Reply(200).
		JSON(map[string]any{"data": map[string]any{
			"resolveReviewThread": map[string]any{"thread": map[string]any{"id": ""}},
		}})

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	err = client.ResolveThread("THREAD_1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing thread id")
}

func TestGetReviewThreadsHandlesNotFoundAndPagination(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		t.Cleanup(gock.Off)

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
				"repository": map[string]any{"pullRequest": nil},
			}})

		client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
		require.NoError(t, err)

		_, err = client.GetReviewThreads("octo", "repo", 12)
		require.ErrorIs(t, err, errNotFound)
	})

	t.Run("paginates", func(t *testing.T) {
		t.Cleanup(gock.Off)

		gock.New("https://api.github.com").
			Post("/graphql").
			AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return false, err
				}
				s := string(body)
				return strings.Contains(s, "reviewThreads(first:100, after:$after)") && strings.Contains(s, "\"after\":null"), nil
			}).
			Reply(200).
			JSON(map[string]any{"data": map[string]any{
				"repository": map[string]any{
					"pullRequest": map[string]any{
						"reviewThreads": map[string]any{
							"nodes": []map[string]any{{
								"id":         "T1",
								"path":       "a.go",
								"line":       1,
								"diffSide":   "RIGHT",
								"isOutdated": false,
								"isResolved": false,
								"comments": map[string]any{
									"nodes":    []map[string]any{{"databaseId": 1, "body": "a", "createdAt": "2025-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}}},
									"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
								},
							}},
							"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "c1"},
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
				s := string(body)
				return strings.Contains(s, "reviewThreads(first:100, after:$after)") && strings.Contains(s, "\"after\":\"c1\""), nil
			}).
			Reply(200).
			JSON(map[string]any{"data": map[string]any{
				"repository": map[string]any{
					"pullRequest": map[string]any{
						"reviewThreads": map[string]any{
							"nodes": []map[string]any{{
								"id":         "T2",
								"path":       "b.go",
								"line":       2,
								"diffSide":   "RIGHT",
								"isOutdated": false,
								"isResolved": false,
								"comments": map[string]any{
									"nodes":    []map[string]any{{"databaseId": 2, "body": "b", "createdAt": "2025-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}}},
									"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
								},
							}},
							"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
						},
					},
				},
			}})

		client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
		require.NoError(t, err)

		threads, err := client.GetReviewThreads("octo", "repo", 12)
		require.NoError(t, err)
		require.Len(t, threads, 2)
		require.Equal(t, "T1", threads[0].ID)
		require.Equal(t, "T2", threads[1].ID)
		require.True(t, gock.IsDone())
	})

	t.Run("thread comment pagination", func(t *testing.T) {
		t.Cleanup(gock.Off)

		gock.New("https://api.github.com").
			Post("/graphql").
			AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return false, err
				}
				s := string(body)
				return strings.Contains(s, "reviewThreads(first:100, after:$after)") && strings.Contains(s, "\"after\":null"), nil
			}).
			Reply(200).
			JSON(map[string]any{"data": map[string]any{
				"repository": map[string]any{
					"pullRequest": map[string]any{
						"reviewThreads": map[string]any{
							"nodes": []map[string]any{{
								"id":         "T1",
								"path":       "a.go",
								"line":       1,
								"diffSide":   "RIGHT",
								"isOutdated": false,
								"isResolved": false,
								"comments": map[string]any{
									"nodes": []map[string]any{
										{"databaseId": 9, "body": "a", "createdAt": "2025-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}},
									},
									"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "c1"},
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
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return false, err
				}
				s := string(body)
				return strings.Contains(s, "node(id:$threadId)") && strings.Contains(s, "\"after\":\"c1\""), nil
			}).
			Reply(200).
			JSON(map[string]any{"data": map[string]any{
				"node": map[string]any{
					"comments": map[string]any{
						"nodes": []map[string]any{
							{"databaseId": 1, "body": "b", "createdAt": "2025-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}},
						},
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					},
				},
			}})

		client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
		require.NoError(t, err)

		threads, err := client.GetReviewThreads("octo", "repo", 12)
		require.NoError(t, err)
		require.Len(t, threads, 1)
		require.Len(t, threads[0].Comments, 2)
		require.Equal(t, int64(1), threads[0].Comments[0].ID)
		require.Equal(t, int64(9), threads[0].Comments[1].ID)
		require.True(t, gock.IsDone())
	})
}

func TestGetThreadCommentsPagePaginatesAndNotFound(t *testing.T) {
	t.Run("node not found", func(t *testing.T) {
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
			JSON(map[string]any{"data": map[string]any{"node": nil}})

		client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
		require.NoError(t, err)

		_, err = client.getThreadCommentsPage("THREAD_1", nil)
		require.ErrorIs(t, err, errNotFound)
	})

	t.Run("paginates", func(t *testing.T) {
		t.Cleanup(gock.Off)

		gock.New("https://api.github.com").
			Post("/graphql").
			AddMatcher(func(req *http.Request, _ *gock.Request) (bool, error) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return false, err
				}
				return strings.Contains(string(body), "\"after\":null"), nil
			}).
			Reply(200).
			JSON(map[string]any{"data": map[string]any{
				"node": map[string]any{
					"comments": map[string]any{
						"nodes":    []map[string]any{{"databaseId": 1, "body": "a", "createdAt": "2025-01-01T00:00:00Z", "author": map[string]any{"login": "bot"}}},
						"pageInfo": map[string]any{"hasNextPage": true, "endCursor": "c1"},
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
				return strings.Contains(string(body), "\"after\":\"c1\""), nil
			}).
			Reply(200).
			JSON(map[string]any{"data": map[string]any{
				"node": map[string]any{
					"comments": map[string]any{
						"nodes":    []map[string]any{{"databaseId": 2, "body": "b", "createdAt": "2025-01-01T00:00:01Z", "author": map[string]any{"login": "bot"}}},
						"pageInfo": map[string]any{"hasNextPage": false, "endCursor": nil},
					},
				},
			}})

		client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
		require.NoError(t, err)

		comments, err := client.getThreadCommentsPage("THREAD_1", nil)
		require.NoError(t, err)
		require.Len(t, comments, 2)
		require.Equal(t, int64(1), comments[0].ID)
		require.Equal(t, int64(2), comments[1].ID)
		require.True(t, gock.IsDone())
	})
}
