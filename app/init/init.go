package init

import (
	"errors"
	"fmt"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zshell"
)

func Clone(dir, name, branch string) (err error) {
	url := "https://github.com/" + name
	code := 0
	outStr := ""
	errStr := ""
	cmd := fmt.Sprintf("git clone -b %s --depth=1 %s %s", branch, url, dir)
	code, outStr, errStr, err = zshell.Run(cmd)
	if code != 0 {
		if outStr != "" {
			err = errors.New(outStr)
		} else if errStr != "" {
			err = errors.New(errStr)
		} else {
			err = errors.New("download failed, please check if the network is normal")
		}
	}
	if err != nil {
		return
	}
	zfile.Rmdir(dir + "/.git")
	zfile.Rmdir(dir + "/.github")

	return
}
