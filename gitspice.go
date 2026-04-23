package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var stackIDRE = regexp.MustCompile(`^#(\d+)$`)

var runExternalCommand = func(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func runGitSpiceLog() (string, string, error) {
	return runExternalCommand("git-spice", "log", "short", "--json")
}

func discoverStackPRs(run func() (string, string, error)) ([]int, []string, error) {
	stdout, stderr, err := run()
	if err != nil {
		if strings.TrimSpace(stderr) != "" {
			return nil, []string{fmt.Sprintf("warning: git-spice failed (%s)", strings.TrimSpace(stderr))}, fmt.Errorf("git-spice failed: %w", err)
		}
		return nil, []string{"warning: git-spice failed"}, fmt.Errorf("git-spice failed: %w", err)
	}

	lines := strings.Split(stdout, "\n")
	prs := make([]int, 0)
	warnings := make([]string, 0)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var row struct {
			Change struct {
				ID string `json:"id"`
			} `json:"change"`
		}
		if unmarshalErr := json.Unmarshal([]byte(trimmed), &row); unmarshalErr != nil {
			warnings = append(warnings, fmt.Sprintf("warning: git-spice JSON parse failed on line %d; ignored", i+1))
			continue
		}
		m := stackIDRE.FindStringSubmatch(strings.TrimSpace(row.Change.ID))
		if len(m) != 2 {
			continue
		}
		n, convErr := strconv.Atoi(m[1])
		if convErr != nil || n <= 0 {
			continue
		}
		prs = append(prs, n)
	}
	return prs, warnings, nil
}
