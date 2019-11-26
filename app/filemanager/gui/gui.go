package gui

import (
	"os"

	"log"
	"os/exec"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

// Register copy/paste file resource
type Register struct {
	MoveSources []*Entry
	CopySources []*Entry
	CopySource  *Entry
}

// ClearMoveResources clear resources
func (r *Register) ClearMoveResources() {
	r.MoveSources = []*Entry{}
}

// ClearCopyResources clear resouces
func (r *Register) ClearCopyResources() {
	r.MoveSources = []*Entry{}
}

// Gui gui have some manager
type Gui struct {
	enablePreview  bool
	InputPath      *tview.InputField
	Register       *Register
	HistoryManager *HistoryManager
	EntryManager   *EntryManager
	Preview        *Preview
	CmdLine        *CmdLine
	App            *tview.Application
	Pages          *tview.Pages
}

func hasEntry(gui *Gui) bool {
	if len(gui.EntryManager.Entries()) != 0 {
		return true
	}
	return false
}

// New create new gui
func New(enablePreview bool, enableIgnorecase bool) *Gui {
	gui := &Gui{
		enablePreview:  enablePreview,
		InputPath:      tview.NewInputField().SetLabel("path").SetLabelWidth(5),
		EntryManager:   NewEntryManager(enableIgnorecase),
		HistoryManager: NewHistoryManager(),
		CmdLine:        NewCmdLine(),
		App:            tview.NewApplication(),
		Register:       &Register{},
	}

	if enablePreview {
		gui.Preview = NewPreview()
	}

	return gui
}

// ExecCmd execute command
func (gui *Gui) ExecCmd(attachStd bool, cmd string, args ...string) error {
	command := exec.Command(cmd, args...)

	if attachStd {
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
	}

	return command.Run()
}

// Stop stop ff
func (gui *Gui) Stop() {
	gui.App.Stop()
}

func (gui *Gui) Message(message string, page tview.Primitive) {
	doneLabel := "ok"
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{doneLabel}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			gui.Pages.RemovePage("message").SwitchToPage("main")
			gui.App.SetFocus(page)
		})

	gui.Pages.AddAndSwitchToPage("message", gui.Modal(modal, 80, 29), true).ShowPage("main")
}

func (gui *Gui) Confirm(message, doneLabel string, page tview.Primitive, doneFunc func() error) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{doneLabel, "cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			gui.Pages.RemovePage("message").SwitchToPage("main")

			if buttonLabel == doneLabel {
				gui.App.QueueUpdateDraw(func() {
					if err := doneFunc(); err != nil {
						log.Println(err)
						gui.Message(err.Error(), page)
					} else {
						gui.App.SetFocus(page)
					}
				})
			}
			gui.App.SetFocus(page)
		})
	gui.Pages.AddAndSwitchToPage("confirm", gui.Modal(modal, 50, 29), true).ShowPage("main")
}

func (gui *Gui) Modal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewGrid().
		SetColumns(0, width, 0).
		SetRows(0, height, 0).
		AddItem(p, 1, 1, 1, 1, 0, 0, true)
}

func (gui *Gui) FocusPanel(p tview.Primitive) {
	gui.App.SetFocus(p)
}

func (gui *Gui) Form(fieldLabel map[string]string, doneLabel, title, pageName string, panel tview.Primitive,
	height int, doneFunc func(values map[string]string) error) {

	form := tview.NewForm()
	for k, v := range fieldLabel {
		form.AddInputField(k, v, 0, nil, nil)
	}

	form.AddButton(doneLabel, func() {
		values := make(map[string]string)

		for label, _ := range fieldLabel {
			item := form.GetFormItemByLabel(label)
			switch item.(type) {
			case *tview.InputField:
				input, ok := item.(*tview.InputField)
				if ok {
					values[label] = os.ExpandEnv(input.GetText())
				}
			}
		}

		if err := doneFunc(values); err != nil {
			log.Println(err)
			gui.Message(err.Error(), gui.EntryManager)
			return
		}

		defer gui.FocusPanel(panel)
		defer gui.Pages.RemovePage(pageName)
	}).
		AddButton("cancel", func() {
			gui.Pages.RemovePage(pageName)
			gui.FocusPanel(panel)
		})

	form.SetBorder(true).SetTitle(title).
		SetTitleAlign(tview.AlignLeft)

	gui.Pages.AddAndSwitchToPage(pageName, gui.Modal(form, 0, height), true).ShowPage("main")
}

// Run run ff
func (gui *Gui) Run() error {
	// get current path
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("%s: %s\n", ErrGetCwd, err)
		return err
	}

	gui.InputPath.SetText(currentDir)

	gui.HistoryManager.Save(0, currentDir)
	gui.EntryManager.SetEntries(currentDir)

	gui.EntryManager.Select(1, 0)

	gui.SetKeybindings()

	grid := tview.NewGrid().SetRows(1, 0, 1).
		AddItem(gui.InputPath, 0, 0, 1, 2, 0, 0, true).
		AddItem(gui.CmdLine, 2, 0, 1, 2, 0, 0, true)

	if gui.enablePreview {
		grid.SetColumns(0, 0).
			AddItem(gui.EntryManager, 1, 0, 1, 1, 0, 0, true).
			AddItem(gui.Preview, 1, 1, 1, 1, 0, 0, true)

		gui.Preview.UpdateView(gui, gui.EntryManager.GetSelectEntry())
	} else {
		grid.AddItem(gui.EntryManager, 1, 0, 1, 2, 0, 0, true)
	}

	gui.Pages = tview.NewPages().
		AddAndSwitchToPage("main", grid, true)

	if err := gui.App.SetRoot(gui.Pages, true).SetFocus(gui.EntryManager).Run(); err != nil {
		gui.App.Stop()
		return err
	}

	return nil
}

func (gui *Gui) Search() {
	pageName := "search"
	if gui.Pages.HasPage(pageName) {
		gui.Pages.ShowPage(pageName)
	} else {
		input := tview.NewInputField()
		input.SetBorder(true).SetTitle("search").SetTitleAlign(tview.AlignLeft)
		input.SetChangedFunc(func(text string) {
			gui.EntryManager.SetSearchWord(text)
			current := gui.InputPath.GetText()
			gui.EntryManager.SetEntries(current)

			if gui.enablePreview {
				gui.Preview.UpdateView(gui, gui.EntryManager.GetSelectEntry())
			}
		})
		input.SetLabel("word").SetLabelWidth(5).SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				gui.Pages.HidePage(pageName)
				gui.FocusPanel(gui.EntryManager)
			}

		})

		gui.Pages.AddAndSwitchToPage(pageName, gui.Modal(input, 0, 3), true).ShowPage("main")
	}
}
