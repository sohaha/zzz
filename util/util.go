package util

import (
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zlsgo/zutil"
)

const (
	CfgFilepath = ".zzz/"
	CfgFilename = "config"
	CfgFileExt  = ".yaml"
)

var (
	Log            *zlog.Logger
	once           sync.Once
	installPath    string
	homePath       string
	Version        = "1.0.44"
	BuildTime      = ""
	BuildGoVersion = ""
)

func init() {
	once.Do(func() {
		Log = zlog.New()
		Log.ResetFlags(zlog.BitLevel)
		homePath, _ = homedir.Dir()
		var defInstallPath string
		var installName string
		envPath := zutil.Getenv("PATH")
		envPath = strings.Split(envPath, ":")[0]
		if zutil.IsWin() {
			defInstallPath = zutil.Getenv("SystemRoot", "C:\\windows") + "\\system32"
			installName = "\\zzz.exe"
		} else {
			defInstallPath = "/usr/local/bin"
			installName = "/zzz"
		}
		if !zfile.DirExist(defInstallPath) {
			defInstallPath = envPath
		}
		installPath = defInstallPath + installName
	})
}

func IsInstall() (exist bool) {
	path := GetInstallPath()
	exist = zfile.FileExist(path)
	if !exist && !zutil.IsWin() {
		exist = zfile.FileExist("/usr/bin/zzz")
	}
	return
}

func GetInstallPath() string {
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

func GetHome() string {
	return homePath + "/"
}

func judge(osName string) (ok bool) {
	switch osName {
	case "win", "windows", "w":
		ok = zutil.IsWin()
	case "mac", "macOS", "macos", "m":
		ok = zutil.IsMac()
	case "linux", "l":
		ok = zutil.IsLinux()
	}
	return
}

func OSCommand(command string) (ncommand string) {
	str := strings.Split(command, "@")
	if len(str) < 2 {
		return command
	}

	ok := false
	switch str[0] {
	case "win", "windows", "w", "mac", "macOS", "macos", "m", "linux", "l":
		ok = judge(str[0])
	default:
		if strings.Contains(str[0], "|") && (strings.Contains(str[0], "w") || strings.Contains(str[0], "m") || strings.Contains(str[0], "l")) {
			for _, v := range strings.Split(str[0], "|") {
				ok = judge(v)
				if ok {
					break
				}
			}
			if !ok {
				return ""
			}
		} else {
			return command
		}
	}

	if ok {
		command = strings.Join(str[1:], "@")
	} else {
		command = ""
	}

	return command
}
