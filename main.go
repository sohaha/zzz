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
	var c = make(chan struct{}, 1)
	go cmd.GetNewVersion(c)
	cmd.Execute()
	<-c
}
