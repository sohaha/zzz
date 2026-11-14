package main

import (
	"github.com/sohaha/zlsgo/zcli"
	"github.com/sohaha/zzz/cmd"
	"github.com/sohaha/zzz/util"
)

func init() {
	zcli.Version = util.Version
}

func main() {
	go cmd.GetNewVersion()
	cmd.Execute()
}
