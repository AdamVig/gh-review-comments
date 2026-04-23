package main

import (
	"regexp"
	"strconv"
	"strings"
)

var suppressedDetailsRE = regexp.MustCompile(`(?is)<details>\s*<summary>\s*Comments suppressed due to low confidence\s*\(\d+\)\s*</summary>(.*?)</details>`)
var suppressedHeaderRE = regexp.MustCompile(`^\*\*(.+):(\d+)\*\*\s*$`)

type parsedSuppressed struct {
	Path  string
	Line  int
	Index int
	Body  string
}

func parseSuppressedComments(reviewBody string) []parsedSuppressed {
	blocks := suppressedDetailsRE.FindAllStringSubmatch(reviewBody, -1)
	if len(blocks) == 0 {
		return nil
	}

	out := make([]parsedSuppressed, 0)
	index := 1
	for _, block := range blocks {
		if len(block) < 2 {
			continue
		}
		items, nextIndex := parseSuppressedBlock(block[1], index)
		index = nextIndex
		out = append(out, items...)
	}
	return out
}

func parseSuppressedBlock(block string, startIndex int) ([]parsedSuppressed, int) {
	lines := strings.Split(block, "\n")
	out := make([]parsedSuppressed, 0)
	index := startIndex

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		m := suppressedHeaderRE.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		path := strings.TrimSpace(m[1])
		lineNum, err := strconv.Atoi(strings.TrimSpace(m[2]))
		if err != nil || lineNum <= 0 || path == "" {
			continue
		}

		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) {
			continue
		}
		bullet := strings.TrimSpace(lines[j])
		bodyText, ok := parseBulletText(bullet)
		if !ok {
			continue
		}

		k := j + 1
		for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
			k++
		}

		body := bodyText
		if k < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[k]), "```") {
			fenceStart := k
			fenceEnd := k
			for fenceEnd+1 < len(lines) {
				fenceEnd++
				if strings.HasPrefix(strings.TrimSpace(lines[fenceEnd]), "```") {
					break
				}
			}
			fence := strings.Join(lines[fenceStart:fenceEnd+1], "\n")
			body = body + "\n" + fence
			i = fenceEnd
		} else {
			i = j
		}

		out = append(out, parsedSuppressed{Path: path, Line: lineNum, Index: index, Body: body})
		index++
	}
	return out, index
}

func parseBulletText(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "* ") && !strings.HasPrefix(trimmed, "**") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "* ")), true
	}
	if after, ok := strings.CutPrefix(trimmed, "- "); ok {
		return strings.TrimSpace(after), true
	}
	return "", false
}
