package main

import "github.com/cli/go-gh/v2/pkg/repository"

func (a *app) runResolve(args []string) (any, *appError) {
	if len(args) == 0 {
		return nil, newUsageError("missing comment-id", "usage: gh review-comments resolve <comment-id> [--repo OWNER/REPO]", nil)
	}
	commentID, parseErr := parseInt64Arg(args[0], "comment-id")
	if parseErr != nil {
		return nil, parseErr
	}

	flags := newFlagSet("resolve")
	var repoArg string
	flags.StringVar(&repoArg, "repo", "", "repository OWNER/REPO")
	if err := flags.Parse(args[1:]); err != nil {
		return nil, parseFlagError(err, "run `gh review-comments resolve --help`")
	}
	if len(flags.Args()) > 0 {
		return nil, newUsageError("unexpected positional arguments", "usage: gh review-comments resolve <comment-id> [--repo OWNER/REPO]", map[string]any{"args": flags.Args()})
	}

	var repoCtx repository.Repository
	var err *appError
	if repoArg != "" {
		repoCtx, err = parseRepo(repoArg, a.repoParse)
		if err != nil {
			return nil, err
		}
	} else {
		repoCtx, err = inferRepo(a.repoCurrent)
		if err != nil {
			return nil, &appError{Code: "repo", Message: "repository context required", Hint: "pass --repo OWNER/REPO", Details: err.Details, Exit: exitError}
		}
	}

	comment, getErr := a.ghapi.GetComment(repoCtx.Owner, repoCtx.Name, commentID)
	if getErr != nil {
		return nil, classifyAPIError(getErr, map[string]any{"repo": repoString(repoCtx), "commentId": commentID, "endpoint": "pulls.comments.get"})
	}

	threadID, threadErr := a.ghapi.FindThreadIDByComment(repoCtx.Owner, repoCtx.Name, comment.PRNumber, commentID)
	if threadErr != nil {
		return nil, classifyAPIError(threadErr, map[string]any{"repo": repoString(repoCtx), "commentId": commentID, "pr": comment.PRNumber, "endpoint": "graphql.reviewThreads.find"})
	}

	resolveErr := a.ghapi.ResolveThread(threadID)
	if resolveErr != nil {
		return nil, classifyAPIError(resolveErr, map[string]any{"repo": repoString(repoCtx), "commentId": commentID, "pr": comment.PRNumber, "threadId": threadID, "endpoint": "graphql.resolveReviewThread"})
	}

	return resolveOutput{
		OK:        true,
		Action:    "resolve",
		Repo:      repoString(repoCtx),
		PR:        comment.PRNumber,
		CommentID: commentID,
		ThreadID:  threadID,
	}, nil
}
