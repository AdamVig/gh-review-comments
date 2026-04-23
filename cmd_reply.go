package main

import "github.com/cli/go-gh/v2/pkg/repository"

const replyUsage = "usage: gh review-comments reply <comment-id> --message-file <path> [--no-resolve] [--repo OWNER/REPO]"

func (a *app) runReply(args []string) (any, *appError) {
	if len(args) == 0 {
		return nil, newUsageError("missing comment-id", replyUsage, nil)
	}
	commentID, parseErr := parseInt64Arg(args[0], "comment-id")
	if parseErr != nil {
		return nil, parseErr
	}

	flags := newFlagSet("reply")
	var messageFile string
	var repoArg string
	var noResolve bool
	flags.StringVar(&messageFile, "message-file", "", "message file path")
	flags.StringVar(&repoArg, "repo", "", "repository OWNER/REPO")
	flags.BoolVar(&noResolve, "no-resolve", false, "reply without resolving the thread")
	if err := flags.Parse(args[1:]); err != nil {
		return nil, parseFlagError(err, "run `gh review-comments reply --help`")
	}
	if len(flags.Args()) > 0 {
		return nil, newUsageError("unexpected positional arguments", replyUsage, map[string]any{"args": flags.Args()})
	}

	if messageFile == "" {
		return nil, newUsageError("missing --message-file", replyUsage, nil)
	}

	body, err := readBodyFromFile(messageFile)
	if err != nil {
		return nil, err
	}

	var repoCtx repository.Repository
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

	createdID, createErr := a.ghapi.CreateReply(repoCtx.Owner, repoCtx.Name, comment.PRNumber, commentID, body)
	if createErr != nil {
		return nil, classifyAPIError(createErr, map[string]any{"repo": repoString(repoCtx), "commentId": commentID, "pr": comment.PRNumber, "endpoint": "pulls.comments.reply"})
	}

	out := replyOutput{
		OK:               true,
		Action:           "reply",
		Repo:             repoString(repoCtx),
		PR:               comment.PRNumber,
		InReplyTo:        commentID,
		CreatedCommentID: createdID,
	}

	if !noResolve {
		threadID, threadErr := a.ghapi.FindThreadIDByComment(repoCtx.Owner, repoCtx.Name, comment.PRNumber, commentID)
		if threadErr != nil {
			return nil, classifyAPIError(threadErr, map[string]any{"repo": repoString(repoCtx), "commentId": commentID, "pr": comment.PRNumber, "createdCommentId": createdID, "endpoint": "graphql.reviewThreads.find"})
		}
		resolveErr := a.ghapi.ResolveThread(threadID)
		if resolveErr != nil {
			return nil, classifyAPIError(resolveErr, map[string]any{"repo": repoString(repoCtx), "commentId": commentID, "pr": comment.PRNumber, "createdCommentId": createdID, "threadId": threadID, "endpoint": "graphql.resolveReviewThread"})
		}
		out.ThreadID = threadID
	}

	return out, nil
}
