package util

import (
	"github.com/sohaha/zlsgo/zenv"
	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var (
	Log *zlog.Logger

	once        sync.Once
	installPath string
)

func init() {
	Log = zlog.New()
	Log.ResetFlags(zlog.BitLevel)
}

func IsInstall() bool {
	path := GetInstallPath()
	return zfile.FileExist(path)
}

func GetInstallPath() string {
	once.Do(func() {
		var defInstallPath string
		var installName string
		envPath := zenv.Getenv("PATH")
		envPath = strings.Split(envPath, ":")[0]
		if zenv.IsWin() {
			defInstallPath = zenv.Getenv("SystemRoot", "C:\\windows") + "\\system32"
			installName = "\\zzz.exe"
		} else {
			installPath = "/usr/local/bin"
			installName = "/zzz"
		}
		if !zfile.DirExist(defInstallPath) {
			defInstallPath = envPath
		}
		installPath = defInstallPath + installName
	})
	return installPath
}

func ExecCommand(commandName string, arg ...string) (string, error) {
	var data string
	c := exec.Command(commandName, arg...)
	c.Env = os.Environ()
	out, err := c.CombinedOutput()
	if out != nil {
		data = zstring.Bytes2String(out)
	}
	if err != nil {
		return data, err
	}
	return data, nil
}

func CheckIfError(err error) {
	if err == nil {
		return
	}

	Log.Fatal(err)
}
