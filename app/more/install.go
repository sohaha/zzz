package more

import (
	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zutil"
	"github.com/sohaha/zzz/util"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (m *Methods) Install(vars []string) {
	path := util.GetInstallPath()
	// if utils.IsInstall() {
	// 	util.Log.Fatalf("the path %s has exist, does not support install", path)
	// }
	// if err := os.MkdirAll(path, 0755); err != nil {
	// 	util.Log.Fatalf(err.Error())
	// }
	ePath, _ := exec.LookPath(os.Args[0])
	ePath, _ = filepath.Abs(ePath)
	res, err := copyMain(ePath, path)
	if err != nil {
		if strings.Contains(res, "Permission denied") {
			util.Log.Fatal("Permission denied, Please use root to execute")
		} else if strings.Contains(err.Error(), "exit status 1") {
			util.Log.Fatal("Installation failed, please try again")
		} else {
			util.Log.Fatal(err)
		}
	}

	util.Log.Success("The installation is complete")
}

func copyMain(src, dest string) (data string, err error) {
	if zutil.IsWin() {
		data, err = util.ExecCommand("cmd", "/C", "copy", src, dest)
	} else {
		data, err = util.ExecCommand("cp", src, dest)
	}
	return
}

func copyFile(src, dest string) (w int64, err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer srcFile.Close()
	destSplitPathDirs := strings.Split(dest, string(filepath.Separator))

	destSplitPath := ""
	for index, dir := range destSplitPathDirs {
		if index < len(destSplitPathDirs)-1 {
			destSplitPath = destSplitPath + dir + string(filepath.Separator)
			i, _ := zfile.PathExist(destSplitPath)
			if i == 0 {
				err := os.Mkdir(destSplitPath, os.ModePerm)
				if err != nil {
					util.Log.Error(err)
				}
			}
		}
	}
	dstFile, err := os.Create(dest)
	if err != nil {
		return
	}
	defer dstFile.Close()

	return io.Copy(dstFile, srcFile)
}
