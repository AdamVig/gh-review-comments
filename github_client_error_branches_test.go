package main

import (
	"errors"
	"net/http"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/require"
	gock "gopkg.in/h2non/gock.v1"
)

func TestGitHubClientMethodFactoryErrorPaths(t *testing.T) {
	restErrClient := &gitHubClient{
		restNew: func() (*api.RESTClient, error) { return nil, errors.New("rest init failed") },
	}
	_, err := restErrClient.GetPR("octo", "repo", 1)
	require.Error(t, err)
	_, err = restErrClient.GetComment("octo", "repo", 1)
	require.Error(t, err)
	_, err = restErrClient.CreateReply("octo", "repo", 1, 1, "x")
	require.Error(t, err)
	_, err = restErrClient.GetReviewBodies("octo", "repo", 1)
	require.Error(t, err)

	gqlErrClient := &gitHubClient{
		graphqlNew: func() (*api.GraphQLClient, error) { return nil, errors.New("gql init failed") },
	}
	_, err = gqlErrClient.GetReviewThreads("octo", "repo", 1)
	require.Error(t, err)
	_, err = gqlErrClient.FindThreadIDByComment("octo", "repo", 1, 1)
	require.Error(t, err)
	err = gqlErrClient.ResolveThread("T1")
	require.Error(t, err)
}

func TestGetReviewBodiesDecodeError(t *testing.T) {
	t.Cleanup(gock.Off)

	gock.New("https://api.github.com").
		Get("/repos/octo/repo/pulls/12/reviews").
		MatchParam("per_page", "100").
		Reply(200).
		BodyString("{")

	client, err := newGitHubClientWithOptions(api.ClientOptions{Host: "github.com", AuthToken: "token", Transport: http.DefaultTransport})
	require.NoError(t, err)

	_, err = client.GetReviewBodies("octo", "repo", 12)
	require.Error(t, err)
}
