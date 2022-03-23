package main

import (
	"github.com/sohaha/zzz/cmd"
)

func main() {
	var c = make(chan struct{}, 0)
	go cmd.GetNewVersion(c)
	cmd.Execute()
	// <-c
}
