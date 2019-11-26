package gui

import (
	"bytes"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/rivo/tview"
)

type Preview struct {
	*tview.TextView
	lineOffset int
}

func NewPreview() *Preview {
	p := &Preview{
		TextView: tview.NewTextView(),
	}

	p.SetBorder(true).SetTitle("preview").SetTitleAlign(tview.AlignLeft)
	p.SetDynamicColors(true)
	return p
}

func (p *Preview) UpdateView(g *Gui, entry *Entry) {
	if entry == nil {
		return
	}

	p.lineOffset = 0

	var text string
	// TODO configrable max file size with option
	// max size is 2MB
	if entry.Size > 200000 && !entry.IsDir {
		text = "file too big"
	} else if !entry.IsDir {
		text = p.Highlight(entry)
	} else {
		text = p.dirEntry(entry.PathName)
	}
	g.App.QueueUpdateDraw(func() {
		p.SetText(text).ScrollToBeginning()
	})
}

func (p *Preview) dirEntry(path string) string {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Println(err)
		return err.Error()
	}

	var contents []string
	for _, f := range files {
		contents = append(contents, f.Name())
	}

	return strings.Join(contents, "\n")
}

func (p *Preview) Highlight(entry *Entry) string {
	// Determine lexer.
	b, err := ioutil.ReadFile(entry.PathName)
	if err != nil {
		log.Println(err)
		return err.Error()
	}

	ext := filepath.Ext(entry.Name)
	l := lexers.Get(ext)
	if l == nil {
		l = lexers.Analyse(string(b))
	}
	if l == nil {
		l = lexers.Fallback
	}
	l = chroma.Coalesce(l)

	// Determine formatter.
	// TODO check terminal
	f := formatters.Get("terminal256")
	if f == nil {
		f = formatters.Fallback
	}

	// Determine style.
	s := styles.Get("monokai")
	if s == nil {
		s = styles.Fallback
	}

	it, err := l.Tokenise(nil, string(b))
	if err != nil {
		log.Println(err)
		return err.Error()
	}

	var buf = bytes.Buffer{}

	if err := f.Format(&buf, s, it); err != nil {
		log.Println(err)
		return err.Error()
	}

	return tview.TranslateANSI(buf.String())
}

func (p *Preview) ScrollDown() {
	// get max offset
	orow, ocol := p.TextView.GetScrollOffset()
	maxOffset, _ := p.TextView.ScrollToEnd().GetScrollOffset()
	// restore offset
	p.TextView.ScrollToBeginning()
	p.TextView.ScrollTo(orow, ocol)

	if p.lineOffset > maxOffset {
		return
	}
	p.lineOffset++
	p.TextView.ScrollTo(p.lineOffset, 0)
}

func (p *Preview) ScrollUp() {
	if p.lineOffset == 0 {
		return
	}
	p.lineOffset--
	p.TextView.ScrollTo(p.lineOffset, 0)
}
