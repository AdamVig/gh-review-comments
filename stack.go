package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

var runExternalCommand = func(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func runGHStackView() (string, string, error) {
	return runExternalCommand("gh", "stack", "view", "--json")
}

func discoverStackPRs(run func() (string, string, error)) ([]int, error) {
	stdout, stderr, err := run()
	if err != nil {
		message := "gh stack view --json failed"
		if strings.TrimSpace(stderr) != "" {
			message = fmt.Sprintf("%s (%s)", message, strings.TrimSpace(stderr))
		}
		return nil, fmt.Errorf("%s: %w", message, err)
	}

	var stack struct {
		Branches []struct {
			IsMerged bool `json:"isMerged"`
			PR       *struct {
				Number int    `json:"number"`
				State  string `json:"state"`
			} `json:"pr"`
		} `json:"branches"`
	}
	if err := json.Unmarshal([]byte(stdout), &stack); err != nil {
		return nil, fmt.Errorf("failed to parse gh stack view --json output: %w", err)
	}

	prs := make([]int, 0, len(stack.Branches))
	for _, branch := range stack.Branches {
		if branch.PR != nil && branch.PR.Number > 0 && !branch.IsMerged && branch.PR.State != "MERGED" {
			prs = append(prs, branch.PR.Number)
		}
	}
	return prs, nil
}
