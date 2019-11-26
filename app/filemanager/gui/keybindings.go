package gui

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell"
	"github.com/sohaha/zzz/app/filemanager/system"
)

var (
	ErrNoDirName       = errors.New("no directory name")
	ErrNoFileName      = errors.New("no file name")
	ErrNoFileOrDirName = errors.New("no file or directory name")
	ErrNoFileOrDir     = errors.New("no file or directory")
	ErrNoNewName       = errors.New("no new name")
)

func (gui *Gui) SetKeybindings() {
	gui.InputPathKeybinding()
	gui.EntryManagerKeybinding()
	gui.CmdLineKeybinding()
}

// globalKeybinding
func (gui *Gui) GlobalKeybinding(event *tcell.EventKey) {
	switch {
	// go to input view
	case event.Key() == tcell.KeyTab:
		gui.App.SetFocus(gui.InputPath)

	// go to previous history
	//case event.Key() == tcell.KeyCtrlH:
	//	history := gui.HistoryManager.Previous()
	//	if history != nil {
	//		gui.InputPath.SetText(history.Path)
	//		gui.EntryManager.SetEntries(history.Path)
	//		gui.EntryManager.Select(history.RowIdx, 0)
	//	}

	//// go to next history
	//case event.Key() == tcell.KeyCtrlL:
	//	history := gui.HistoryManager.Next()
	//	if history != nil {
	//		gui.InputPath.SetText(history.Path)
	//		gui.EntryManager.SetEntries(history.Path)
	//		gui.EntryManager.Select(history.RowIdx, 0)
	//	}

	// go to parent dir
	case event.Rune() == 'h':
		current := gui.InputPath.GetText()
		parent := filepath.Dir(current)

		if parent != "" {
			// save select position
			gui.EntryManager.SetSelectPos(current)

			// update entries
			gui.InputPath.SetText(parent)
			gui.EntryManager.SetEntries(parent)
			gui.EntryManager.SetOffset(0, 0)

			// restore select position
			gui.EntryManager.RestorePos(parent)

			if gui.enablePreview {
				entry := gui.EntryManager.GetSelectEntry()
				gui.Preview.UpdateView(gui, entry)
			}
		}

	// go to selected dir
	case event.Rune() == 'l':
		entry := gui.EntryManager.GetSelectEntry()

		if entry != nil && entry.IsDir {
			// save select position
			gui.EntryManager.SetSelectPos(gui.InputPath.GetText())
			gui.EntryManager.SetEntries(entry.PathName)

			gui.InputPath.SetText(entry.PathName)

			gui.EntryManager.RestorePos(entry.PathName)

			row, _ := gui.EntryManager.GetSelection()
			count := gui.EntryManager.GetRowCount()
			if row > count {
				gui.EntryManager.Select(count-1, 0)
			}

			if gui.enablePreview {
				entry := gui.EntryManager.GetSelectEntry()
				gui.Preview.UpdateView(gui, entry)
			}
		}
	}
}

func (gui *Gui) EntryManagerKeybinding() {
	gui.EntryManager.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			gui.App.Stop()
		}

	}).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		gui.GlobalKeybinding(event)

		if gui.enablePreview {
			switch event.Key() {
			case tcell.KeyCtrlJ:
				gui.Preview.ScrollDown()
			case tcell.KeyCtrlK:
				gui.Preview.ScrollUp()
			}
		}

		switch event.Rune() {
		case 'd':
			if !hasEntry(gui) {
				return event
			}

			gui.Confirm("do you want to remove this?", "yes", gui.EntryManager, func() error {
				entry := gui.EntryManager.GetSelectEntry()
				if entry == nil {
					return nil
				}

				if entry.IsDir {
					if err := system.RemoveDirAll(entry.PathName); err != nil {
						log.Println(err)
						return err
					}
				} else {
					if err := system.RemoveFile(entry.PathName); err != nil {
						log.Println(err)
						return err
					}
				}

				path := gui.InputPath.GetText()
				gui.EntryManager.SetEntries(path)
				return nil
			})

		// copy entry
		case 'y':
			if !hasEntry(gui) {
				return event
			}

			m := gui.EntryManager
			m.UpdateColor()
			entry := m.GetSelectEntry()
			gui.Register.CopySource = entry

			row, _ := m.GetSelection()
			for i := 0; i < 5; i++ {
				m.GetCell(row, i).SetTextColor(tcell.ColorYellow)
			}

		// paste entry
		case 'p':
			if gui.Register.CopySource != nil {
				source := gui.Register.CopySource

				gui.Form(map[string]string{"name": source.Name}, "paste", "new name", "new_name", gui.EntryManager,
					7, func(values map[string]string) error {
						name := values["name"]
						if name == "" {
							return ErrNoNewName
						}

						target := filepath.Join(gui.InputPath.GetText(), name)
						if err := system.Copy(source.PathName, target); err != nil {
							log.Println(err)
							return err
						}

						gui.Register.CopySource = nil
						gui.EntryManager.SetEntries(gui.InputPath.GetText())
						return nil
					})
			}

		// edit file with $EDITOR
		case 'e':
			editor := os.Getenv("EDITOR")
			if editor == "" {
				log.Println("$EDITOR is empty, please set $EDITOR")
				return event
			}

			entry := gui.EntryManager.GetSelectEntry()
			if entry == nil {
				log.Println("cannot get entry")
				return event
			}

			gui.App.Suspend(func() {
				if err := gui.ExecCmd(true, editor, entry.PathName); err != nil {
					log.Printf("%s: %s\n", ErrEdit, err)
				}
			})

			if gui.enablePreview {
				entry := gui.EntryManager.GetSelectEntry()
				gui.Preview.UpdateView(gui, entry)
			}
		case 'm':
			gui.Form(map[string]string{"name": ""}, "create", "new direcotry",
				"create_directory", gui.EntryManager,
				7, func(values map[string]string) error {
					name := values["name"]
					if name == "" {
						return ErrNoDirName
					}

					target := filepath.Join(gui.InputPath.GetText(), name)
					if err := system.NewDir(target); err != nil {
						log.Println(err)
						return err
					}

					gui.EntryManager.SetEntries(gui.InputPath.GetText())
					return nil
				})
		case 'r':
			entry := gui.EntryManager.GetSelectEntry()
			if entry == nil {
				return event
			}

			gui.Form(map[string]string{"new name": entry.Name}, "rename", "new name", "rename", gui.EntryManager,
				7, func(values map[string]string) error {
					name := values["new name"]
					if name == "" {
						return ErrNoFileName
					}

					current := gui.InputPath.GetText()

					target := filepath.Join(current, name)
					if err := system.Rename(entry.PathName, target); err != nil {
						return err
					}

					gui.EntryManager.SetEntries(gui.InputPath.GetText())
					return nil
				})

		case 'n':
			gui.Form(map[string]string{"name": ""}, "create", "new file", "create_file", gui.EntryManager,
				7, func(values map[string]string) error {
					name := values["name"]
					if name == "" {
						return ErrNoFileOrDirName
					}

					target := filepath.Join(gui.InputPath.GetText(), name)
					if err := system.NewFile(target); err != nil {
						log.Println(err)
						return err
					}

					gui.EntryManager.SetEntries(gui.InputPath.GetText())
					return nil
				})
		case 'q':
			gui.Stop()

		case 'o':
			entry := gui.EntryManager.GetSelectEntry()
			if entry == nil {
				return event
			}
			if err := system.Open(entry.PathName); err != nil {
				gui.Message(err.Error(), gui.EntryManager)
			}

		case 'f', '/':
			gui.Search()
		case ':', 'c':
			gui.FocusPanel(gui.CmdLine)
		}

		return event
	})

	gui.EntryManager.SetSelectionChangedFunc(func(row, col int) {
		if row > 0 {
			if gui.enablePreview {
				f := gui.EntryManager.Entries()[row-1]
				gui.Preview.UpdateView(gui, f)
			}
		}
	})

}

func (gui *Gui) InputPathKeybinding() {
	gui.InputPath.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			gui.App.Stop()
		}

		if key == tcell.KeyEnter {
			path := gui.InputPath.GetText()
			path = os.ExpandEnv(path)
			gui.InputPath.SetText(path)
			//row, _ := gui.EntryManager.GetSelection()
			//gui.HistoryManager.Save(row, path)
			gui.EntryManager.SetEntries(path)
			gui.FocusPanel(gui.EntryManager)
		}

	}).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			gui.App.SetFocus(gui.EntryManager)
		}

		return event
	})
}

func (gui *Gui) CmdLineKeybinding() {
	cmdline := gui.CmdLine

	cmdline.SetDoneFunc(func(key tcell.Key) {
		text := cmdline.GetText()
		if text == "" {
			return
		}

		cmdText := strings.Split(text, " ")

		// expand environments
		for i, c := range cmdText[1:] {
			cmdText[i+1] = os.ExpandEnv(c)
		}

		cmd := exec.Command(cmdText[0], cmdText[1:]...)

		buf := bytes.Buffer{}
		cmd.Stderr = &buf
		cmd.Stdout = &buf
		if err := cmd.Run(); err == nil {
			cmdline.SetText("")
		}

		result := strings.TrimRight(buf.String(), "\n")
		if result != "" {
			gui.Message(result, cmdline)
		}

		gui.EntryManager.SetEntries(gui.InputPath.GetText())
	}).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyEsc:
			gui.App.SetFocus(gui.EntryManager)
			return event
		}

		return event
	})
}
