package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
)

var prURLRE = regexp.MustCompile(`^https?://[^/]+/([^/]+)/([^/]+)/pull/(\d+)(?:[/?#].*)?$`)

type prTarget struct {
	Number int
	Repo   repository.Repository
}

type batchListResult struct {
	data    map[int]ghListPRData
	perErr  map[int]error
	callErr error
}

func (a *app) runList(args []string) (any, *appError) {
	flags := newFlagSet("list")
	var prFlags stringSliceFlag
	var authorFlags stringSliceFlag
	var repoArg string
	var maxBody intFlag
	flags.Var(&prFlags, "pr", "PR number or PR URL")
	flags.Var(&authorFlags, "author", "author login filter")
	flags.StringVar(&repoArg, "repo", "", "repository OWNER/REPO")
	flags.Var(&maxBody, "max-body", "truncate body fields to N runes")
	if err := flags.Parse(args); err != nil {
		return nil, parseFlagError(err, "run `gh review-comments list --help`")
	}
	if len(flags.Args()) > 0 {
		return nil, newUsageError("unexpected positional arguments", "use only flags for `list`", map[string]any{"args": flags.Args()})
	}
	if maxBody.set && maxBody.value < 0 {
		return nil, newUsageError("invalid --max-body", "pass --max-body >= 0", map[string]any{"maxBody": maxBody.value})
	}
	var maxBodyPtr *int
	if maxBody.set {
		maxBodyPtr = &maxBody.value
	}
	authors := normalizeAuthors(authorFlags.items)
	authorSet := authorsSet(authors)

	var (
		targets  []prTarget
		scope    string
		baseRepo repository.Repository
		haveBase bool
	)

	if repoArg != "" {
		repo, err := parseRepo(repoArg, a.repoParse)
		if err != nil {
			return nil, err
		}
		baseRepo = repo
		haveBase = true
	}

	if len(prFlags.items) > 0 {
		scope = "explicit"
		targets = make([]prTarget, 0, len(prFlags.items))
		for _, raw := range prFlags.items {
			num, parsedRepo, fromURL, parseErr := parsePRRef(raw)
			if parseErr != nil {
				return nil, &appError{Code: "usage", Message: "invalid --pr value", Hint: "pass --pr as a number or PR URL", Details: map[string]any{"value": raw}, Exit: exitUsage}
			}
			if fromURL {
				if haveBase && !sameRepo(baseRepo, parsedRepo) {
					return nil, newUsageError(
						"mixed repositories in --pr values",
						"use one repository per command or pass --repo OWNER/REPO",
						map[string]any{
							"value":    raw,
							"expected": repoString(baseRepo),
							"actual":   repoString(parsedRepo),
						},
					)
				}
				if !haveBase {
					baseRepo = parsedRepo
					haveBase = true
				}
				targets = append(targets, prTarget{Number: num, Repo: baseRepo})
				continue
			}
			if !haveBase {
				inferred, err := inferRepo(a.repoCurrent)
				if err != nil {
					targets = append(targets, prTarget{Number: num, Repo: repository.Repository{}})
					continue
				}
				baseRepo = inferred
				haveBase = true
			}
			targets = append(targets, prTarget{Number: num, Repo: baseRepo})
		}
	} else {
		scope = "current"
		ghArgs := []string{"pr", "view", "--json", "number,headRepository,headRepositoryOwner"}
		if repoArg != "" {
			ghArgs = append(ghArgs, "--repo", repoArg)
		}
		out, errOut, err := a.ghExec(ghArgs...)
		if err != nil {
			return nil, &appError{
				Code:    "repo",
				Message: "could not infer current pull request",
				Hint:    "pass --pr <number|url> explicitly",
				Details: map[string]any{"stderr": strings.TrimSpace(errOut), "error": err.Error()},
				Exit:    exitError,
			}
		}
		number, inferredRepo, parseErr := parseCurrentPRFromJSON(out)
		if parseErr != nil {
			return nil, &appError{Code: "parse", Message: "failed to parse current pull request", Hint: "retry with `gh pr view --json number,headRepository,headRepositoryOwner`", Details: map[string]any{"error": parseErr.Error()}, Exit: exitError}
		}
		if !haveBase {
			baseRepo = inferredRepo
			haveBase = true
		}
		targets = []prTarget{{Number: number, Repo: baseRepo}}
	}

	return a.buildListOutput(targets, scope, authors, authorSet, baseRepo, haveBase, maxBodyPtr)
}

func (a *app) runListStack(args []string) (any, *appError) {
	flags := newFlagSet("list-stack")
	var authorFlags stringSliceFlag
	var repoArg string
	var maxBody intFlag
	flags.Var(&authorFlags, "author", "author login filter")
	flags.StringVar(&repoArg, "repo", "", "repository OWNER/REPO")
	flags.Var(&maxBody, "max-body", "truncate body fields to N runes")
	if err := flags.Parse(args); err != nil {
		return nil, parseFlagError(err, "run `gh review-comments list-stack --help`")
	}
	if len(flags.Args()) > 0 {
		return nil, newUsageError("unexpected positional arguments", "use only flags for `list-stack`", map[string]any{"args": flags.Args()})
	}
	if maxBody.set && maxBody.value < 0 {
		return nil, newUsageError("invalid --max-body", "pass --max-body >= 0", map[string]any{"maxBody": maxBody.value})
	}
	var maxBodyPtr *int
	if maxBody.set {
		maxBodyPtr = &maxBody.value
	}
	authors := normalizeAuthors(authorFlags.items)
	authorSet := authorsSet(authors)

	var baseRepo repository.Repository
	var haveBase bool
	if repoArg != "" {
		repo, err := parseRepo(repoArg, a.repoParse)
		if err != nil {
			return nil, err
		}
		baseRepo = repo
		haveBase = true
	}

	stackPRs, warnings, stackErr := discoverStackPRs(a.gitSpiceLog)
	for _, warning := range warnings {
		fmt.Fprintln(a.stderr, warning)
	}
	if stackErr != nil {
		return nil, &appError{
			Code:    "repo",
			Message: "could not infer stack pull requests",
			Hint:    "use `list` with --pr values instead",
			Details: map[string]any{"error": stackErr.Error()},
			Exit:    exitError,
		}
	}
	if len(stackPRs) == 0 {
		return nil, &appError{
			Code:    "repo",
			Message: "no stack pull requests found",
			Hint:    "use `list` with --pr values instead",
			Exit:    exitError,
		}
	}
	if !haveBase {
		repo, err := inferRepo(a.repoCurrent)
		if err != nil {
			return nil, err
		}
		baseRepo = repo
		haveBase = true
	}
	targets := make([]prTarget, 0, len(stackPRs))
	for _, n := range stackPRs {
		targets = append(targets, prTarget{Number: n, Repo: baseRepo})
	}

	return a.buildListOutput(targets, "stack", authors, authorSet, baseRepo, haveBase, maxBodyPtr)
}

func (a *app) buildListOutput(targets []prTarget, scope string, authors []string, authorSet map[string]struct{}, baseRepo repository.Repository, haveBase bool, maxBodyPtr *int) (any, *appError) {
	prs := make([]prOutput, 0, len(targets))
	successCount := 0
	var firstErr *appError
	resolvedRepoForOutput := ""
	if haveBase {
		resolvedRepoForOutput = repoString(baseRepo)
	}
	batchResults := a.prefetchBatchListData(targets)

	for _, target := range targets {
		prEntry := prOutput{Number: target.Number, Threads: []threadOut{}, Suppressed: []suppressed{}}
		if target.Repo.Owner == "" || target.Repo.Name == "" {
			prEntry.Error = &prErrorOut{Code: "repo", Message: "could not infer repository", Hint: "pass --repo OWNER/REPO"}
			prs = append(prs, prEntry)
			if firstErr == nil {
				firstErr = &appError{Code: "repo", Message: "could not infer repository", Hint: "pass --repo OWNER/REPO", Exit: exitError}
			}
			continue
		}
		if resolvedRepoForOutput == "" {
			resolvedRepoForOutput = repoString(target.Repo)
		}

		repoKey := batchRepoKey(target.Repo)
		if batch, ok := batchResults[repoKey]; ok && batch.callErr == nil {
			if batch.perErr != nil {
				if dataErr, ok := batch.perErr[target.Number]; ok {
					classified := classifyAPIError(dataErr, map[string]any{"repo": repoString(target.Repo), "pr": target.Number, "endpoint": "graphql.list.batch"})
					prEntry.Error = &prErrorOut{Code: classified.Code, Message: classified.Message, Hint: classified.Hint}
					prs = append(prs, prEntry)
					if firstErr == nil {
						firstErr = classified
					}
					continue
				}
			}
			if data, ok := batch.data[target.Number]; ok {
				prs = append(prs, formatPRFromData(target.Number, data, authorSet, maxBodyPtr))
				successCount++
				continue
			}
			classified := classifyAPIError(errNotFound, map[string]any{"repo": repoString(target.Repo), "pr": target.Number, "endpoint": "graphql.list.batch"})
			prEntry.Error = &prErrorOut{Code: classified.Code, Message: classified.Message, Hint: classified.Hint}
			prs = append(prs, prEntry)
			if firstErr == nil {
				firstErr = classified
			}
			continue
		}

		scanned, err := a.scanPR(target.Repo, target.Number, authorSet, maxBodyPtr)
		if err != nil {
			prEntry.Error = &prErrorOut{Code: err.Code, Message: err.Message, Hint: err.Hint}
			prs = append(prs, prEntry)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		prs = append(prs, scanned)
		successCount++
	}

	if successCount == 0 {
		if firstErr == nil {
			firstErr = &appError{Code: "api", Message: "failed to scan pull requests", Hint: "verify repository and pull request visibility", Exit: exitError}
		}
		return nil, &appError{
			Code:    firstErr.Code,
			Message: "failed to scan all pull requests",
			Hint:    firstErr.Hint,
			Details: map[string]any{"count": len(targets)},
			Exit:    exitError,
		}
	}

	return listOutput{
		OK:      true,
		Repo:    resolvedRepoForOutput,
		Scope:   scope,
		Authors: authors,
		PRs:     prs,
	}, nil
}

func (a *app) prefetchBatchListData(targets []prTarget) map[string]batchListResult {
	type repoRequest struct {
		repo    repository.Repository
		numbers []int
		seen    map[int]struct{}
	}
	grouped := map[string]*repoRequest{}
	for _, target := range targets {
		if target.Number <= 0 || target.Repo.Owner == "" || target.Repo.Name == "" {
			continue
		}
		key := batchRepoKey(target.Repo)
		req := grouped[key]
		if req == nil {
			req = &repoRequest{
				repo:    target.Repo,
				numbers: []int{},
				seen:    map[int]struct{}{},
			}
			grouped[key] = req
		}
		if _, ok := req.seen[target.Number]; ok {
			continue
		}
		req.seen[target.Number] = struct{}{}
		req.numbers = append(req.numbers, target.Number)
	}

	out := make(map[string]batchListResult, len(grouped))
	for key, req := range grouped {
		data, perErr, err := a.ghapi.ListPRData(req.repo.Owner, req.repo.Name, req.numbers)
		out[key] = batchListResult{
			data:    data,
			perErr:  perErr,
			callErr: err,
		}
	}
	return out
}

func batchRepoKey(repo repository.Repository) string {
	return strings.ToLower(repo.Owner) + "/" + strings.ToLower(repo.Name)
}

func (a *app) scanPR(repo repository.Repository, number int, authors map[string]struct{}, maxBody *int) (prOutput, *appError) {
	prInfo, err := a.ghapi.GetPR(repo.Owner, repo.Name, number)
	if err != nil {
		return prOutput{}, classifyAPIError(err, map[string]any{"repo": repoString(repo), "pr": number, "endpoint": "pulls.get"})
	}
	threads, err := a.ghapi.GetReviewThreads(repo.Owner, repo.Name, number)
	if err != nil {
		return prOutput{}, classifyAPIError(err, map[string]any{"repo": repoString(repo), "pr": number, "endpoint": "graphql.reviewThreads"})
	}
	reviews, err := a.ghapi.GetReviewBodies(repo.Owner, repo.Name, number)
	if err != nil {
		return prOutput{}, classifyAPIError(err, map[string]any{"repo": repoString(repo), "pr": number, "endpoint": "pulls.reviews.list"})
	}
	return formatPRFromData(number, ghListPRData{
		PR:      prInfo,
		Threads: threads,
		Reviews: reviews,
	}, authors, maxBody), nil
}

func formatPRFromData(number int, data ghListPRData, authors map[string]struct{}, maxBody *int) prOutput {
	outThreads := make([]threadOut, 0)
	for _, t := range data.Threads {
		if t.IsResolved {
			continue
		}
		include := false
		for _, c := range t.Comments {
			if authorMatches(authors, c.Author) {
				include = true
				break
			}
		}
		if !include {
			continue
		}
		anchor := int64(0)
		if len(t.Comments) > 0 {
			anchor = t.Comments[0].ID
		}
		comments := make([]commentOut, 0, len(t.Comments))
		for _, c := range t.Comments {
			comments = append(comments, commentOut{ID: c.ID, Author: c.Author, Body: truncateRunes(c.Body, maxBody)})
		}
		outThreads = append(outThreads, threadOut{
			Path:            t.Path,
			Line:            t.Line,
			Side:            t.Side,
			IsOutdated:      t.IsOutdated,
			AnchorCommentID: anchor,
			Comments:        comments,
		})
	}

	sort.SliceStable(outThreads, func(i, j int) bool {
		if outThreads[i].Path != outThreads[j].Path {
			return outThreads[i].Path < outThreads[j].Path
		}
		if outThreads[i].Line != outThreads[j].Line {
			return outThreads[i].Line < outThreads[j].Line
		}
		if outThreads[i].Side != outThreads[j].Side {
			return outThreads[i].Side < outThreads[j].Side
		}
		return outThreads[i].AnchorCommentID < outThreads[j].AnchorCommentID
	})

	suppressedItems := make([]suppressed, 0)
	latest, parsed := latestSuppressedFromReviews(data.Reviews, authors)
	if latest != nil {
		for _, item := range parsed {
			suppressedItems = append(suppressedItems, suppressed{
				Path:     item.Path,
				Line:     item.Line,
				ReviewID: latest.ReviewID,
				Index:    item.Index,
				Body:     truncateRunes(item.Body, maxBody),
			})
		}
	}

	return prOutput{
		Number:     number,
		Title:      truncateRunes(data.PR.Title, maxBody),
		URL:        data.PR.URL,
		Threads:    outThreads,
		Suppressed: suppressedItems,
	}
}

func latestSuppressedFromReviews(reviews []ghReviewBody, authors map[string]struct{}) (*ghReviewBody, []parsedSuppressed) {
	var (
		best       *ghReviewBody
		bestParsed []parsedSuppressed
	)
	for i := range reviews {
		if !authorMatches(authors, reviews[i].Author) {
			continue
		}
		parsed := parseSuppressedComments(reviews[i].Body)
		if len(parsed) == 0 {
			continue
		}
		if best == nil || isNewerReview(reviews[i], *best) {
			best = &reviews[i]
			bestParsed = parsed
		}
	}
	return best, bestParsed
}

func isNewerReview(a, b ghReviewBody) bool {
	if a.SubmittedAt != b.SubmittedAt {
		if a.SubmittedAt == "" {
			return false
		}
		if b.SubmittedAt == "" {
			return true
		}
		return a.SubmittedAt > b.SubmittedAt
	}
	return a.ReviewID > b.ReviewID
}

func parsePRRef(raw string) (number int, repo repository.Repository, fromURL bool, err error) {
	if n, convErr := strconv.Atoi(raw); convErr == nil && n > 0 {
		return n, repository.Repository{}, false, nil
	}
	m := prURLRE.FindStringSubmatch(raw)
	if len(m) == 0 {
		return 0, repository.Repository{}, false, fmt.Errorf("invalid PR reference")
	}
	n, convErr := strconv.Atoi(m[3])
	if convErr != nil || n <= 0 {
		return 0, repository.Repository{}, false, fmt.Errorf("invalid PR number")
	}
	return n, repository.Repository{Owner: m[1], Name: m[2], Host: "github.com"}, true, nil
}

func sameRepo(a, b repository.Repository) bool {
	return strings.EqualFold(a.Owner, b.Owner) && strings.EqualFold(a.Name, b.Name)
}
