package build

import (
	"os/exec"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zlsgo/ztime"
	"github.com/sohaha/zlsgo/ztype"
	"github.com/sohaha/zlsgo/zutil"
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

func DisabledCGO() bool {
	cgo := zutil.Getenv("CGO_ENABLED")

	if cgo == "" {
		_, s, _, _ := zshell.Run("go env CGO_ENABLED")
		cgo = zstring.TrimSpace(s)
	}

	return !ztype.ToBool(cgo)
}
