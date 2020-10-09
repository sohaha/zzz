package init

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sohaha/gconf"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zshell"

	"github.com/sohaha/zzz/util"
)

type (
	stInitConf struct {
		Command []string
		Dir     string
	}
)

var conf stInitConf

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
	// zfile.Rmdir(dir + "/.github")

	if initConf(dir) {
		initCommand(dir)
	}

	return
}

func initConf(dir string) bool {
	commandFile := dir + "/zzz-init.yaml"
	if !zfile.FileExist(commandFile) {
		return false
	}
	defer zfile.Rmdir(commandFile)
	cfg := gconf.New(commandFile)
	err := cfg.Read()
	if err == nil {
		err = cfg.Unmarshal(&conf)
	}
	if err == nil {
		conf.Dir = dir
	}
	if err != nil {
		util.Log.Warn("init conf err:", err)
	}
	return true
}

func initCommand(dir string) {
	if len(conf.Command) > 0 {
		for _, v := range conf.Command {
			command := util.OSCommand(v)
			if command == "" {
				// util.Log.Info("ignore command:", v)
				continue
			}
			cmd := strings.Split(command, "&&")
			for _, v := range cmd {
				ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
				c := strings.Trim(v, " ")
				util.Log.Info("Conmand:", c)
				zshell.Dir = dir
				code, _, errMsg, err := zshell.RunContext(ctx, c)
				if errMsg != "" {
					util.Log.Println(errMsg)
				}
				if err != nil || code != 0 {
					util.Log.Error("Fatal:", c)
					break
				}
			}
		}
		zshell.Dir = ""
	}
}
