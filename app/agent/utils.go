package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zlsgo/ztime"
)

func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("时长字符串为空")
	}

	var total time.Duration
	re := regexp.MustCompile(`(\d+)([hms])`)
	indexes := re.FindAllStringIndex(s, -1)
	if len(indexes) == 0 {
		return 0, fmt.Errorf("无效的时长格式")
	}

	prevEnd := 0
	for _, idx := range indexes {
		if idx[0] != prevEnd {
			return 0, fmt.Errorf("无效的时长格式")
		}
		prevEnd = idx[1]
	}
	if prevEnd != len(s) {
		return 0, fmt.Errorf("无效的时长格式")
	}

	matches := re.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		val, _ := strconv.Atoi(match[1])
		switch match[2] {
		case "h":
			total += time.Duration(val) * time.Hour
		case "m":
			total += time.Duration(val) * time.Minute
		case "s":
			total += time.Duration(val) * time.Second
		}
	}

	return total, nil
}

func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	var result string
	if hours > 0 {
		result += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		result += fmt.Sprintf("%dm", minutes)
	}
	if seconds > 0 || result == "" {
		result += fmt.Sprintf("%ds", seconds)
	}

	return result
}

func DetectGitHubRepo() (owner, repo string, err error) {
	code, stdout, _, err := zshell.ExecCommand(context.Background(), []string{"git", "remote", "get-url", "origin"}, nil, nil, nil)
	if err != nil || code != 0 {
		return "", "", fmt.Errorf("获取 git remote 失败")
	}

	remoteURL := strings.TrimSpace(stdout)

	httpsRe := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)
	sshRe := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)

	if matches := httpsRe.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return matches[1], matches[2], nil
	}
	if matches := sshRe.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return matches[1], matches[2], nil
	}

	return "", "", fmt.Errorf("无法解析 GitHub URL")
}

func CheckHasChanges() (bool, error) {
	code, output, _, _ := zshell.ExecCommand(context.Background(),
		[]string{"git", "status", "--porcelain"}, nil, nil, nil)

	if code != 0 {
		return false, fmt.Errorf("git status 失败")
	}

	return strings.TrimSpace(output) != "", nil
}

func CleanupBranch(branchName, mainBranch string) {
	if branchName == "" {
		return
	}
	zshell.ExecCommand(context.Background(), []string{"git", "checkout", mainBranch}, nil, nil, nil)
	zshell.ExecCommand(context.Background(), []string{"git", "branch", "-D", branchName}, nil, nil, nil)
}

func GetCurrentBranch() string {
	code, stdout, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "rev-parse", "--abbrev-ref", "HEAD"}, nil, nil, nil)
	if code != 0 {
		return "main"
	}
	return strings.TrimSpace(stdout)
}

func GetHeadSHA() (string, error) {
	code, stdout, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "rev-parse", "HEAD"}, nil, nil, nil)
	if code != 0 {
		return "", fmt.Errorf("获取 HEAD 失败")
	}
	return strings.TrimSpace(stdout), nil
}

func GenerateRandomHash() string {
	return zstring.Rand(8)
}

func GetIterationDisplay(iteration, maxRuns, extraIterations int) func() string {
	if maxRuns == 0 {
		return func() string {
			return fmt.Sprintf("(%d)", iteration)
		}
	}
	total := maxRuns + extraIterations
	return func() string {
		now := ztime.Now("H:i:s")
		return fmt.Sprintf("%s (%d/%d)", now, iteration, total)
	}
}
