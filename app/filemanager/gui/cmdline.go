package gui

import (
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

type CmdLine struct {
	*tview.InputField
}

func NewCmdLine() *CmdLine {
	c := &CmdLine{
		InputField: tview.NewInputField(),
	}

	c.SetLabel("cmd:")
	c.SetFieldBackgroundColor(tcell.ColorBlack)
	return c
}
