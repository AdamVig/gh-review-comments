package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

var errNotFound = errors.New("not found")

var nextLinkRE = regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)"`)

type gitHubClient struct {
	rest       *api.RESTClient
	graphql    *api.GraphQLClient
	restNew    func() (*api.RESTClient, error)
	graphqlNew func() (*api.GraphQLClient, error)
}

func newGitHubClient() *gitHubClient {
	return &gitHubClient{
		restNew:    api.DefaultRESTClient,
		graphqlNew: api.DefaultGraphQLClient,
	}
}

func newGitHubClientWithOptions(opts api.ClientOptions) (*gitHubClient, error) {
	rest, err := api.NewRESTClient(opts)
	if err != nil {
		return nil, err
	}
	graphql, err := api.NewGraphQLClient(opts)
	if err != nil {
		return nil, err
	}
	return &gitHubClient{rest: rest, graphql: graphql}, nil
}

func (c *gitHubClient) restClient() (*api.RESTClient, error) {
	if c.rest != nil {
		return c.rest, nil
	}
	if c.restNew == nil {
		return nil, errors.New("missing rest client factory")
	}
	cl, err := c.restNew()
	if err != nil {
		return nil, err
	}
	c.rest = cl
	return cl, nil
}

func (c *gitHubClient) gqlClient() (*api.GraphQLClient, error) {
	if c.graphql != nil {
		return c.graphql, nil
	}
	if c.graphqlNew == nil {
		return nil, errors.New("missing graphql client factory")
	}
	cl, err := c.graphqlNew()
	if err != nil {
		return nil, err
	}
	c.graphql = cl
	return cl, nil
}

type ghPR struct {
	Number int
	Title  string
	URL    string
}

type ghThreadComment struct {
	ID        int64
	Author    string
	Body      string
	CreatedAt string
}

type ghThread struct {
	ID         string
	Path       string
	Line       int
	Side       string
	IsOutdated bool
	IsResolved bool
	Comments   []ghThreadComment
}

type ghReviewBody struct {
	ReviewID    int64
	Author      string
	SubmittedAt string
	Body        string
}

type ghComment struct {
	ID       int64
	PRNumber int
}

type ghListPRData struct {
	PR      ghPR
	Threads []ghThread
	Reviews []ghReviewBody
}

const listBatchChunkSize = 10

func (c *gitHubClient) ListPRData(owner, repo string, numbers []int) (map[int]ghListPRData, map[int]error, error) {
	out := make(map[int]ghListPRData, len(numbers))
	perErr := map[int]error{}
	unique := dedupPositiveInts(numbers)
	if len(unique) == 0 {
		return out, perErr, nil
	}

	for i := 0; i < len(unique); i += listBatchChunkSize {
		end := min(i+listBatchChunkSize, len(unique))
		chunk := unique[i:end]

		query, vars, aliasToNumber := buildListBatchGraphQLQuery(owner, repo, chunk)
		gql, err := c.gqlClient()
		if err != nil {
			return nil, nil, err
		}

		var response struct {
			Repository map[string]json.RawMessage `json:"repository"`
		}
		if err := gql.Do(query, vars, &response); err != nil {
			return nil, nil, err
		}
		if response.Repository == nil {
			return nil, nil, errors.New("missing repository in list batch response")
		}

		for alias, prNumber := range aliasToNumber {
			raw, ok := response.Repository[alias]
			if !ok || isNullJSON(raw) {
				perErr[prNumber] = errNotFound
				continue
			}

			var node listBatchPRNode
			if err := json.Unmarshal(raw, &node); err != nil {
				perErr[prNumber] = fmt.Errorf("failed to parse list batch response for PR %d: %w", prNumber, err)
				continue
			}
			if node.Number <= 0 {
				node.Number = prNumber
			}

			threads, err := c.convertBatchThreads(owner, repo, prNumber, node.ReviewThreads)
			if err != nil {
				perErr[prNumber] = err
				continue
			}
			reviews, err := c.convertBatchReviews(owner, repo, prNumber, node.Reviews)
			if err != nil {
				perErr[prNumber] = err
				continue
			}

			out[prNumber] = ghListPRData{
				PR:      ghPR{Number: node.Number, Title: node.Title, URL: node.URL},
				Threads: threads,
				Reviews: reviews,
			}
		}
	}

	return out, perErr, nil
}

type listBatchPRNode struct {
	Number        int                 `json:"number"`
	Title         string              `json:"title"`
	URL           string              `json:"url"`
	ReviewThreads listBatchThreadConn `json:"reviewThreads"`
	Reviews       listBatchReviewConn `json:"reviews"`
}

type listBatchThreadConn struct {
	Nodes    []listBatchThreadNode `json:"nodes"`
	PageInfo listBatchPageInfo     `json:"pageInfo"`
}

type listBatchThreadNode struct {
	ID         string               `json:"id"`
	Path       string               `json:"path"`
	Line       *int                 `json:"line"`
	DiffSide   *string              `json:"diffSide"`
	IsOutdated bool                 `json:"isOutdated"`
	IsResolved bool                 `json:"isResolved"`
	Comments   listBatchCommentConn `json:"comments"`
}

type listBatchCommentConn struct {
	Nodes    []listBatchCommentNode `json:"nodes"`
	PageInfo listBatchPageInfo      `json:"pageInfo"`
}

type listBatchCommentNode struct {
	DatabaseID *int64 `json:"databaseId"`
	Body       string `json:"body"`
	CreatedAt  string `json:"createdAt"`
	Author     *struct {
		Login string `json:"login"`
	} `json:"author"`
}

type listBatchReviewConn struct {
	Nodes    []listBatchReviewNode `json:"nodes"`
	PageInfo listBatchPageInfo     `json:"pageInfo"`
}

type listBatchReviewNode struct {
	DatabaseID  *int64 `json:"databaseId"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submittedAt"`
	Author      *struct {
		Login string `json:"login"`
	} `json:"author"`
}

type listBatchPageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}

func (c *gitHubClient) convertBatchThreads(owner, repo string, prNumber int, conn listBatchThreadConn) ([]ghThread, error) {
	if conn.PageInfo.HasNextPage {
		return c.GetReviewThreads(owner, repo, prNumber)
	}
	out := make([]ghThread, 0, len(conn.Nodes))
	for _, node := range conn.Nodes {
		thread := ghThread{
			ID:         node.ID,
			Path:       node.Path,
			Line:       valueOrZero(node.Line),
			Side:       valueOrEmpty(node.DiffSide),
			IsOutdated: node.IsOutdated,
			IsResolved: node.IsResolved,
			Comments:   make([]ghThreadComment, 0, len(node.Comments.Nodes)),
		}
		for _, cn := range node.Comments.Nodes {
			if cn.DatabaseID == nil {
				continue
			}
			author := ""
			if cn.Author != nil {
				author = cn.Author.Login
			}
			thread.Comments = append(thread.Comments, ghThreadComment{
				ID:        *cn.DatabaseID,
				Author:    author,
				Body:      cn.Body,
				CreatedAt: cn.CreatedAt,
			})
		}
		if node.Comments.PageInfo.HasNextPage {
			extra, err := c.getThreadCommentsPage(node.ID, node.Comments.PageInfo.EndCursor)
			if err != nil {
				return nil, err
			}
			thread.Comments = append(thread.Comments, extra...)
		}
		stabilizeCommentTieOrder(thread.Comments)
		out = append(out, thread)
	}
	return out, nil
}

func (c *gitHubClient) convertBatchReviews(owner, repo string, prNumber int, conn listBatchReviewConn) ([]ghReviewBody, error) {
	if conn.PageInfo.HasNextPage {
		return c.GetReviewBodies(owner, repo, prNumber)
	}
	out := make([]ghReviewBody, 0, len(conn.Nodes))
	for _, node := range conn.Nodes {
		if strings.TrimSpace(node.Body) == "" {
			continue
		}
		author := ""
		if node.Author != nil {
			author = node.Author.Login
		}
		reviewID := int64(0)
		if node.DatabaseID != nil {
			reviewID = *node.DatabaseID
		}
		out = append(out, ghReviewBody{
			ReviewID:    reviewID,
			Author:      author,
			SubmittedAt: node.SubmittedAt,
			Body:        node.Body,
		})
	}
	return out, nil
}

func buildListBatchGraphQLQuery(owner, repo string, numbers []int) (string, map[string]any, map[string]int) {
	var b strings.Builder
	vars := map[string]any{
		"owner": owner,
		"repo":  repo,
	}
	aliasToNumber := make(map[string]int, len(numbers))

	b.WriteString("query($owner:String!, $repo:String!")
	for i, n := range numbers {
		varName := fmt.Sprintf("n%d", i)
		b.WriteString(fmt.Sprintf(", $%s:Int!", varName))
		vars[varName] = n
	}
	b.WriteString(") {\n")
	b.WriteString("  repository(owner:$owner, name:$repo) {\n")
	for i, n := range numbers {
		alias := fmt.Sprintf("pr%d", i)
		varName := fmt.Sprintf("n%d", i)
		aliasToNumber[alias] = n
		b.WriteString(fmt.Sprintf("    %s: pullRequest(number:$%s) {\n", alias, varName))
		b.WriteString(`      number
      title
      url
      reviewThreads(first:100) {
        nodes {
          id
          path
          line
          diffSide
          isOutdated
          isResolved
          comments(first:100) {
            nodes {
              databaseId
              body
              createdAt
              author { login }
            }
            pageInfo { hasNextPage endCursor }
          }
        }
        pageInfo { hasNextPage endCursor }
      }
      reviews(first:100) {
        nodes {
          databaseId
          body
          submittedAt
          author { login }
        }
        pageInfo { hasNextPage endCursor }
      }
    }
`)
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String(), vars, aliasToNumber
}

func dedupPositiveInts(numbers []int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0, len(numbers))
	for _, n := range numbers {
		if n <= 0 {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func isNullJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null"
}

func (c *gitHubClient) GetPR(owner, repo string, number int) (ghPR, error) {
	rest, err := c.restClient()
	if err != nil {
		return ghPR{}, err
	}
	var out struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number)
	if err := rest.Get(path, &out); err != nil {
		return ghPR{}, err
	}
	if out.Number == 0 {
		out.Number = number
	}
	return ghPR{Number: out.Number, Title: out.Title, URL: out.HTMLURL}, nil
}

func (c *gitHubClient) GetReviewThreads(owner, repo string, number int) ([]ghThread, error) {
	gql, err := c.gqlClient()
	if err != nil {
		return nil, err
	}

	threads := make([]ghThread, 0)
	var after *string
	for {
		vars := map[string]any{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"after":  after,
		}

		var response struct {
			Repository struct {
				PullRequest *struct {
					ReviewThreads struct {
						Nodes []struct {
							ID         string  `json:"id"`
							Path       string  `json:"path"`
							Line       *int    `json:"line"`
							DiffSide   *string `json:"diffSide"`
							IsOutdated bool    `json:"isOutdated"`
							IsResolved bool    `json:"isResolved"`
							Comments   struct {
								Nodes []struct {
									DatabaseID *int64 `json:"databaseId"`
									Body       string `json:"body"`
									CreatedAt  string `json:"createdAt"`
									Author     *struct {
										Login string `json:"login"`
									} `json:"author"`
								} `json:"nodes"`
								PageInfo struct {
									HasNextPage bool    `json:"hasNextPage"`
									EndCursor   *string `json:"endCursor"`
								} `json:"pageInfo"`
							} `json:"comments"`
						} `json:"nodes"`
						PageInfo struct {
							HasNextPage bool    `json:"hasNextPage"`
							EndCursor   *string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}

		if err := gql.Do(`query($owner:String!, $repo:String!, $number:Int!, $after:String) {
  repository(owner:$owner, name:$repo) {
    pullRequest(number:$number) {
      reviewThreads(first:100, after:$after) {
        nodes {
          id
          path
          line
          diffSide
          isOutdated
          isResolved
          comments(first:100) {
            nodes {
              databaseId
              body
              createdAt
              author { login }
            }
            pageInfo { hasNextPage endCursor }
          }
        }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}`,
			vars,
			&response,
		); err != nil {
			return nil, err
		}

		if response.Repository.PullRequest == nil {
			return nil, errNotFound
		}

		for _, n := range response.Repository.PullRequest.ReviewThreads.Nodes {
			thread := ghThread{
				ID:         n.ID,
				Path:       n.Path,
				Line:       valueOrZero(n.Line),
				Side:       valueOrEmpty(n.DiffSide),
				IsOutdated: n.IsOutdated,
				IsResolved: n.IsResolved,
				Comments:   make([]ghThreadComment, 0, len(n.Comments.Nodes)),
			}
			for _, cn := range n.Comments.Nodes {
				if cn.DatabaseID == nil {
					continue
				}
				author := ""
				if cn.Author != nil {
					author = cn.Author.Login
				}
				thread.Comments = append(thread.Comments, ghThreadComment{
					ID:        *cn.DatabaseID,
					Author:    author,
					Body:      cn.Body,
					CreatedAt: cn.CreatedAt,
				})
			}
			if n.Comments.PageInfo.HasNextPage {
				extra, err := c.getThreadCommentsPage(n.ID, n.Comments.PageInfo.EndCursor)
				if err != nil {
					return nil, err
				}
				thread.Comments = append(thread.Comments, extra...)
			}
			threads = append(threads, thread)
		}

		if !response.Repository.PullRequest.ReviewThreads.PageInfo.HasNextPage {
			break
		}
		after = response.Repository.PullRequest.ReviewThreads.PageInfo.EndCursor
	}

	for i := range threads {
		stabilizeCommentTieOrder(threads[i].Comments)
	}

	return threads, nil
}

func (c *gitHubClient) getThreadCommentsPage(threadID string, cursor *string) ([]ghThreadComment, error) {
	gql, err := c.gqlClient()
	if err != nil {
		return nil, err
	}

	out := make([]ghThreadComment, 0)
	after := cursor
	for {
		vars := map[string]any{"threadId": threadID, "after": after}
		var response struct {
			Node *struct {
				Comments struct {
					Nodes []struct {
						DatabaseID *int64 `json:"databaseId"`
						Body       string `json:"body"`
						CreatedAt  string `json:"createdAt"`
						Author     *struct {
							Login string `json:"login"`
						} `json:"author"`
					} `json:"nodes"`
					PageInfo struct {
						HasNextPage bool    `json:"hasNextPage"`
						EndCursor   *string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"comments"`
			} `json:"node"`
		}
		if err := gql.Do(`query($threadId:ID!, $after:String) {
  node(id:$threadId) {
    ... on PullRequestReviewThread {
      comments(first:100, after:$after) {
        nodes {
          databaseId
          body
          createdAt
          author { login }
        }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}`,
			vars,
			&response,
		); err != nil {
			return nil, err
		}
		if response.Node == nil {
			return nil, errNotFound
		}
		for _, n := range response.Node.Comments.Nodes {
			if n.DatabaseID == nil {
				continue
			}
			author := ""
			if n.Author != nil {
				author = n.Author.Login
			}
			out = append(out, ghThreadComment{
				ID:        *n.DatabaseID,
				Author:    author,
				Body:      n.Body,
				CreatedAt: n.CreatedAt,
			})
		}
		if !response.Node.Comments.PageInfo.HasNextPage {
			break
		}
		after = response.Node.Comments.PageInfo.EndCursor
	}
	return out, nil
}

func (c *gitHubClient) GetReviewBodies(owner, repo string, number int) ([]ghReviewBody, error) {
	rest, err := c.restClient()
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews?per_page=100", owner, repo, number)
	out := make([]ghReviewBody, 0)
	for {
		resp, err := rest.Request(http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		var page []struct {
			ID          int64  `json:"id"`
			SubmittedAt string `json:"submitted_at"`
			User        *struct {
				Login string `json:"login"`
			} `json:"user"`
			Body string `json:"body"`
		}
		dec := json.NewDecoder(resp.Body)
		decodeErr := dec.Decode(&page)
		closeErr := resp.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		for _, v := range page {
			if strings.TrimSpace(v.Body) == "" {
				continue
			}
			author := ""
			if v.User != nil {
				author = v.User.Login
			}
			out = append(out, ghReviewBody{
				ReviewID:    v.ID,
				Author:      author,
				SubmittedAt: v.SubmittedAt,
				Body:        v.Body,
			})
		}

		next, ok := nextLink(resp.Header.Get("Link"))
		if !ok {
			break
		}
		path = next
	}
	return out, nil
}

func (c *gitHubClient) GetComment(owner, repo string, commentID int64) (ghComment, error) {
	rest, err := c.restClient()
	if err != nil {
		return ghComment{}, err
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/comments/%d", owner, repo, commentID)
	var out struct {
		ID             int64  `json:"id"`
		PullRequestURL string `json:"pull_request_url"`
	}
	if err := rest.Get(path, &out); err != nil {
		return ghComment{}, err
	}
	prNum, err := parsePRNumberFromURL(out.PullRequestURL)
	if err != nil {
		return ghComment{}, err
	}
	return ghComment{ID: out.ID, PRNumber: prNum}, nil
}

func (c *gitHubClient) CreateReply(owner, repo string, prNumber int, inReplyTo int64, body string) (int64, error) {
	rest, err := c.restClient()
	if err != nil {
		return 0, err
	}
	request := map[string]any{"body": body, "in_reply_to": inReplyTo}
	b, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}
	path := fmt.Sprintf("repos/%s/%s/pulls/%d/comments", owner, repo, prNumber)
	var out struct {
		ID int64 `json:"id"`
	}
	if err := rest.Post(path, bytes.NewReader(b), &out); err != nil {
		return 0, err
	}
	if out.ID <= 0 {
		return 0, errors.New("missing created comment id")
	}
	return out.ID, nil
}

func (c *gitHubClient) FindThreadIDByComment(owner, repo string, prNumber int, commentID int64) (string, error) {
	threads, err := c.GetReviewThreads(owner, repo, prNumber)
	if err != nil {
		return "", err
	}
	for _, t := range threads {
		for _, comment := range t.Comments {
			if comment.ID == commentID {
				return t.ID, nil
			}
		}
	}
	return "", errNotFound
}

func (c *gitHubClient) ResolveThread(threadID string) error {
	gql, err := c.gqlClient()
	if err != nil {
		return err
	}
	vars := map[string]any{"threadId": threadID}
	var response struct {
		ResolveReviewThread struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		} `json:"resolveReviewThread"`
	}
	if err := gql.Do(`mutation($threadId:ID!) {
  resolveReviewThread(input:{threadId:$threadId}) {
    thread { id }
  }
}`,
		vars,
		&response,
	); err != nil {
		return err
	}
	if response.ResolveReviewThread.Thread.ID == "" {
		return errors.New("missing thread id in resolve mutation response")
	}
	return nil
}

func nextLink(linkHeader string) (string, bool) {
	if linkHeader == "" {
		return "", false
	}
	matches := nextLinkRE.FindAllStringSubmatch(linkHeader, -1)
	for _, m := range matches {
		if len(m) > 2 && m[2] == "next" {
			return m[1], true
		}
	}
	return "", false
}

func parsePRNumberFromURL(url string) (int, error) {
	idx := strings.LastIndex(url, "/pulls/")
	if idx < 0 {
		return 0, fmt.Errorf("invalid pull_request_url: %s", url)
	}
	tail := url[idx+len("/pulls/"):]
	if slash := strings.IndexByte(tail, '/'); slash >= 0 {
		tail = tail[:slash]
	}
	n, err := strconv.Atoi(tail)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid pull request number in pull_request_url: %s", url)
	}
	return n, nil
}

func valueOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func stabilizeCommentTieOrder(comments []ghThreadComment) {
	if len(comments) < 2 {
		return
	}
	start := 0
	for start < len(comments) {
		end := start + 1
		for end < len(comments) && comments[end].CreatedAt == comments[start].CreatedAt {
			end++
		}
		if end-start > 1 {
			sort.SliceStable(comments[start:end], func(i, j int) bool {
				return comments[start+i].ID < comments[start+j].ID
			})
		}
		start = end
	}
}

func classifyAPIError(err error, details map[string]any) *appError {
	if errors.Is(err, errNotFound) {
		return &appError{Code: "notfound", Message: "resource not found", Hint: "verify repository and identifier", Details: details, Exit: exitError}
	}
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		d := cloneDetails(details)
		d["status"] = httpErr.StatusCode
		switch httpErr.StatusCode {
		case http.StatusUnauthorized:
			return &appError{Code: "auth", Message: "authentication failed", Hint: "run `gh auth status` and authenticate", Details: d, Exit: exitError}
		case http.StatusForbidden:
			return &appError{Code: "forbidden", Message: "access forbidden", Hint: "ensure your token has pull request permissions", Details: d, Exit: exitError}
		case http.StatusNotFound:
			return &appError{Code: "notfound", Message: "resource not found", Hint: "verify repository and identifier", Details: d, Exit: exitError}
		default:
			return &appError{Code: "api", Message: "GitHub API request failed", Hint: "retry with GH_DEBUG=api and inspect stderr", Details: d, Exit: exitError}
		}
	}
	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) {
		d := cloneDetails(details)
		if len(gqlErr.Errors) > 0 {
			d["gqlType"] = gqlErr.Errors[0].Type
			d["gqlMessage"] = gqlErr.Errors[0].Message
			switch gqlErr.Errors[0].Type {
			case "FORBIDDEN":
				return &appError{Code: "forbidden", Message: "access forbidden", Hint: "ensure your token has pull request permissions", Details: d, Exit: exitError}
			case "NOT_FOUND":
				return &appError{Code: "notfound", Message: "resource not found", Hint: "verify repository and identifier", Details: d, Exit: exitError}
			}
		}
		return &appError{Code: "api", Message: "GitHub GraphQL request failed", Hint: "retry with GH_DEBUG=api and inspect stderr", Details: d, Exit: exitError}
	}
	if strings.Contains(strings.ToLower(err.Error()), "authentication") {
		return &appError{Code: "auth", Message: "authentication failed", Hint: "run `gh auth status` and authenticate", Details: details, Exit: exitError}
	}
	return &appError{Code: "internal", Message: "internal runtime error", Hint: "retry; if it persists inspect stderr", Details: mergeDetails(details, map[string]any{"error": err.Error()}), Exit: exitError}
}

func cloneDetails(details map[string]any) map[string]any {
	if details == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(details))
	maps.Copy(out, details)
	return out
}

func mergeDetails(base map[string]any, add map[string]any) map[string]any {
	out := cloneDetails(base)
	maps.Copy(out, add)
	return out
}
