package gui

import (
	"github.com/dustin/go-humanize"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

var (
	dateFmt = "2006-01-02 15:04:05"
)

// Entry file or dir info
type Entry struct {
	Name       string // file name
	Path       string // file path
	PathName   string // file's path and name
	Access     string
	Create     string
	Change     string
	Size       int64
	Permission string
	Owner      string
	Group      string
	Viewable   bool
	IsDir      bool
}

type selectPos struct {
	row int
	col int
}

// EntryManager file list
type EntryManager struct {
	enableIgnorecase bool
	*tview.Table
	entries    []*Entry
	selectPos  map[string]selectPos
	searchWord string
}

// NewEntryManager new entry list
func NewEntryManager(enableIgnorecase bool) *EntryManager {
	e := &EntryManager{
		enableIgnorecase: enableIgnorecase,
		Table:            tview.NewTable().Select(0, 0).SetFixed(1, 1).SetSelectable(true, false),
		selectPos:        make(map[string]selectPos),
	}

	e.SetBorder(true).SetTitle("files").SetTitleAlign(tview.AlignLeft)

	return e
}

// Entries get entries
func (e *EntryManager) Entries() []*Entry {
	return e.entries
}

func (e *EntryManager) SetSearchWord(word string) {
	e.searchWord = word
}

// SetSelectPos save select position
func (e *EntryManager) SetSelectPos(path string) {
	row, col := e.GetSelection()
	e.selectPos[path] = selectPos{row, col}
}

// RestorePos restore select position
func (e *EntryManager) RestorePos(path string) {
	pos, ok := e.selectPos[path]
	if !ok {
		pos = selectPos{1, 0}
	}

	e.Select(pos.row, pos.col)
}

func (e *EntryManager) RefreshView() {
	e.SetColumns()
}

func (e *EntryManager) SetViewable(viewable bool) {
	entry := e.GetSelectEntry()
	entry.Viewable = viewable
	e.RefreshView()
}

// SetHeader set table header
func (e *EntryManager) SetHeader() {
	headers := []string{
		"Name",
		"Size",
		"Permission",
		"Owner",
		"Group",
	}
	for k, v := range headers {
		e.Table.SetCell(0, k, &tview.TableCell{
			Text:            v,
			NotSelectable:   true,
			Align:           tview.AlignLeft,
			Color:           tcell.ColorYellow,
			BackgroundColor: tcell.ColorDefault,
		})
	}
}

// SetColumns set entries
func (e *EntryManager) SetColumns() {
	table := e.Clear()
	e.SetHeader()
	var i int
	for _, entry := range e.entries {
		size := humanize.Bytes(uint64(entry.Size))
		table.SetCell(i+1, 0, tview.NewTableCell(entry.Name))
		table.SetCell(i+1, 1, tview.NewTableCell(size))
		table.SetCell(i+1, 2, tview.NewTableCell(entry.Permission))
		table.SetCell(i+1, 3, tview.NewTableCell(entry.Owner))
		table.SetCell(i+1, 4, tview.NewTableCell(entry.Group))
		i++
	}

	e.UpdateColor()
}

// GetSelectEntry get selected entry
func (e *EntryManager) GetSelectEntry() *Entry {
	row, _ := e.GetSelection()
	if len(e.entries) == 0 {
		return nil
	}
	if row < 1 {
		return nil
	}

	if row > len(e.entries) {
		return nil
	}
	return e.entries[row-1]
}

func (e *EntryManager) UpdateColor() {
	rowNum := e.GetRowCount()

	e.GetSelection()
	for i := 1; i < rowNum; i++ {
		color := tcell.ColorWhite
		if e.Entries()[i-1].IsDir {
			color = tcell.ColorDarkCyan
		}

		for j := 0; j < 5; j++ {
			e.GetCell(i, j).SetTextColor(color)
		}
	}

}
