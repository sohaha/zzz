package util

import (
	"github.com/blang/semver"
	"github.com/mitchellh/go-homedir"
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
	Log         *zlog.Logger
	once        sync.Once
	installPath string
	homePath    string
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
	return homePath
}

func DoSelfUpdate(version string) {
	cmdPath, err := os.Executable()
	if err != nil {
		return
	}
	v, err := semver.Make(version)
	if err != nil {
		Log.Error(err)
		return
	}
	newV, err := semver.Make("1.0.0")
	if err != nil {
		Log.Error(err)
		return
	}
	Log.Debug(v.LTE(newV))
	Log.Debug(v.String())
	Log.Debug(newV.String())
	Log.Debug(cmdPath)
}

func dd() {
	//
	// stat, err := os.Lstat(cmdPath)
	// if err != nil {
	// 	return nil, fmt.Errorf("Failed to stat '%s'. File may not exist: %s", cmdPath, err)
	// }
	// if stat.Mode()&os.ModeSymlink != 0 {
	// 	p, err := filepath.EvalSymlinks(cmdPath)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("Failed to resolve symlink '%s' for executable: %s", cmdPath, err)
	// 	}
	// 	cmdPath = p
	// }
}
