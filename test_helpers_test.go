package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
)

type fakeGitHub struct {
	prs       map[string]ghPR
	threads   map[string][]ghThread
	reviews   map[string][]ghReviewBody
	comments  map[string]ghComment
	replies   []fakeReplyCall
	resolved  []string
	threadByC map[string]string
	errByKey  map[string]error

	listPRDataCalls    int
	getPRCalls         int
	getThreadsCalls    int
	getReviewBodyCalls int
}

type fakeReplyCall struct {
	Owner     string
	Repo      string
	PR        int
	InReplyTo int64
	Body      string
}

func newFakeGitHub() *fakeGitHub {
	return &fakeGitHub{
		prs:       map[string]ghPR{},
		threads:   map[string][]ghThread{},
		reviews:   map[string][]ghReviewBody{},
		comments:  map[string]ghComment{},
		threadByC: map[string]string{},
		errByKey:  map[string]error{},
	}
}

func keyPR(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s#%d", owner, repo, number)
}

func keyComment(owner, repo string, id int64) string {
	return fmt.Sprintf("%s/%s@%d", owner, repo, id)
}

func (f *fakeGitHub) GetPR(owner, repo string, number int) (ghPR, error) {
	f.getPRCalls++
	if err, ok := f.errByKey["GetPR:"+keyPR(owner, repo, number)]; ok {
		return ghPR{}, err
	}
	v, ok := f.prs[keyPR(owner, repo, number)]
	if !ok {
		return ghPR{}, errNotFound
	}
	return v, nil
}

func (f *fakeGitHub) GetReviewThreads(owner, repo string, number int) ([]ghThread, error) {
	f.getThreadsCalls++
	if err, ok := f.errByKey["GetReviewThreads:"+keyPR(owner, repo, number)]; ok {
		return nil, err
	}
	v, ok := f.threads[keyPR(owner, repo, number)]
	if !ok {
		return []ghThread{}, nil
	}
	out := make([]ghThread, len(v))
	copy(out, v)
	return out, nil
}

func (f *fakeGitHub) GetReviewBodies(owner, repo string, number int) ([]ghReviewBody, error) {
	f.getReviewBodyCalls++
	if err, ok := f.errByKey["GetReviewBodies:"+keyPR(owner, repo, number)]; ok {
		return nil, err
	}
	v, ok := f.reviews[keyPR(owner, repo, number)]
	if !ok {
		return []ghReviewBody{}, nil
	}
	out := make([]ghReviewBody, len(v))
	copy(out, v)
	return out, nil
}

func (f *fakeGitHub) ListPRData(owner, repo string, numbers []int) (map[int]ghListPRData, map[int]error, error) {
	f.listPRDataCalls++
	if err, ok := f.errByKey["ListPRDataCall:"+owner+"/"+repo]; ok {
		return nil, nil, err
	}
	data := make(map[int]ghListPRData, len(numbers))
	perErr := map[int]error{}
	for _, number := range numbers {
		prKey := keyPR(owner, repo, number)
		if err, ok := f.errByKey["ListPRData:"+prKey]; ok {
			perErr[number] = err
			continue
		}
		pr, ok := f.prs[prKey]
		if !ok {
			perErr[number] = errNotFound
			continue
		}
		threads := make([]ghThread, len(f.threads[prKey]))
		copy(threads, f.threads[prKey])
		reviews := make([]ghReviewBody, len(f.reviews[prKey]))
		copy(reviews, f.reviews[prKey])
		data[number] = ghListPRData{PR: pr, Threads: threads, Reviews: reviews}
	}
	return data, perErr, nil
}

func (f *fakeGitHub) GetComment(owner, repo string, commentID int64) (ghComment, error) {
	if err, ok := f.errByKey["GetComment:"+keyComment(owner, repo, commentID)]; ok {
		return ghComment{}, err
	}
	v, ok := f.comments[keyComment(owner, repo, commentID)]
	if !ok {
		return ghComment{}, errNotFound
	}
	return v, nil
}

func (f *fakeGitHub) CreateReply(owner, repo string, prNumber int, inReplyTo int64, body string) (int64, error) {
	if err, ok := f.errByKey[fmt.Sprintf("CreateReply:%s/%s#%d@%d", owner, repo, prNumber, inReplyTo)]; ok {
		return 0, err
	}
	f.replies = append(f.replies, fakeReplyCall{Owner: owner, Repo: repo, PR: prNumber, InReplyTo: inReplyTo, Body: body})
	return inReplyTo + 1000, nil
}

func (f *fakeGitHub) FindThreadIDByComment(owner, repo string, prNumber int, commentID int64) (string, error) {
	key := fmt.Sprintf("%s/%s#%d@%d", owner, repo, prNumber, commentID)
	if err, ok := f.errByKey["FindThreadIDByComment:"+key]; ok {
		return "", err
	}
	id, ok := f.threadByC[key]
	if !ok {
		return "", errNotFound
	}
	return id, nil
}

func (f *fakeGitHub) ResolveThread(threadID string) error {
	if err, ok := f.errByKey["ResolveThread:"+threadID]; ok {
		return err
	}
	f.resolved = append(f.resolved, threadID)
	return nil
}

func newTestApp(fake *fakeGitHub) (*app, *strings.Builder, *strings.Builder) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	return &app{
		stdin:  strings.NewReader(""),
		stdout: stdout,
		stderr: stderr,
		ghapi:  fake,
		repoCurrent: func() (repository.Repository, error) {
			return repository.Repository{Host: "github.com", Owner: "octo", Name: "repo"}, nil
		},
		repoParse: repository.Parse,
		ghExec: func(args ...string) (string, string, error) {
			if strings.Join(args, " ") == "pr view --json number,headRepository,headRepositoryOwner" {
				return `{"number":7,"headRepository":{"name":"repo"},"headRepositoryOwner":{"login":"octo"}}`, "", nil
			}
			if strings.HasPrefix(strings.Join(args, " "), "pr view --json number,headRepository,headRepositoryOwner --repo ") {
				return `{"number":7,"headRepository":{"name":"repo"},"headRepositoryOwner":{"login":"octo"}}`, "", nil
			}
			return "", "", fmt.Errorf("unexpected gh args")
		},
		gitSpiceLog: func() (string, string, error) {
			return "", "not tracked", fmt.Errorf("exit status 1")
		},
	}, stdout, stderr
}

func sortedThreadIDs(threads []threadOut) []int64 {
	out := make([]int64, 0, len(threads))
	for _, t := range threads {
		out = append(out, t.AnchorCommentID)
	}
	slices.Sort(out)
	return out
}
