package build

import (
	"os/exec"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/ztime"
)

func GetGoVersion() string {
	cmd := exec.Command("go", "version")
	if out, err := cmd.CombinedOutput(); err == nil {
		goversion := strings.TrimPrefix(strings.TrimSpace(string(out)), "go version ")
		return goversion
	}
	return "None"
}

func GetBuildGitID() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err == nil {
		commitid := strings.TrimSpace(string(out))
		return commitid
	}
	return "None"
}

func GetBuildTime() string {
	return ztime.FormatTime(time.Now())
}
