package main

// failurePayload is the stable error payload for all non-help failures.
type failurePayload struct {
	OK      bool           `toon:"ok"`
	Code    string         `toon:"code"`
	Message string         `toon:"message"`
	Hint    string         `toon:"hint"`
	Details map[string]any `toon:"details"`
}

type listOutput struct {
	OK      bool       `toon:"ok"`
	Repo    string     `toon:"repo"`
	Scope   string     `toon:"scope"`
	Authors []string   `toon:"authors"`
	PRs     []prOutput `toon:"prs"`
}

type prOutput struct {
	Number     int          `toon:"number"`
	Title      string       `toon:"title,omitempty"`
	URL        string       `toon:"url,omitempty"`
	Threads    []threadOut  `toon:"threads"`
	Suppressed []suppressed `toon:"suppressed"`
	Error      *prErrorOut  `toon:"error,omitempty"`
}

type prErrorOut struct {
	Code    string `toon:"code"`
	Message string `toon:"message"`
	Hint    string `toon:"hint"`
}

type threadOut struct {
	Path            string       `toon:"path"`
	Line            int          `toon:"line"`
	Side            string       `toon:"side"`
	IsOutdated      bool         `toon:"isOutdated"`
	AnchorCommentID int64        `toon:"anchorCommentId"`
	Comments        []commentOut `toon:"comments"`
}

type commentOut struct {
	ID     int64  `toon:"id"`
	Author string `toon:"author"`
	Body   string `toon:"body"`
}

type suppressed struct {
	Path     string `toon:"path"`
	Line     int    `toon:"line"`
	ReviewID int64  `toon:"reviewId"`
	Index    int    `toon:"index"`
	Body     string `toon:"body"`
}

type replyOutput struct {
	OK               bool   `toon:"ok"`
	Action           string `toon:"action"`
	Repo             string `toon:"repo"`
	PR               int    `toon:"pr"`
	InReplyTo        int64  `toon:"inReplyTo"`
	CreatedCommentID int64  `toon:"createdCommentId"`
	ThreadID         string `toon:"threadId,omitempty"`
}

type resolveOutput struct {
	OK        bool   `toon:"ok"`
	Action    string `toon:"action"`
	Repo      string `toon:"repo"`
	PR        int    `toon:"pr"`
	CommentID int64  `toon:"commentId"`
	ThreadID  string `toon:"threadId"`
}
