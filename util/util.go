package util

import (
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/sohaha/zlsgo/zenv"
	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
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
	Version        = "1.0.19"
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
		envPath := zenv.Getenv("PATH")
		envPath = strings.Split(envPath, ":")[0]
		if zenv.IsWin() {
			defInstallPath = zenv.Getenv("SystemRoot", "C:\\windows") + "\\system32"
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
	if !exist && !zenv.IsWin() {
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
