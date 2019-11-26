// +build !windows

package gui

import (
	"io/ioutil"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"log"

	"gopkg.in/djherbis/times.v1"
)

// SetEntries set entries
func (e *EntryManager) SetEntries(path string) []*Entry {
	var entries []*Entry

	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Printf("%s: %s\n", ErrReadDir, err)
		return nil
	}

	if len(files) == 0 {
		e.entries = entries
		e.SetColumns()
		return nil
	}

	var access, change, create, perm, owner, group string

	for _, file := range files {
		var name, word string
		if e.enableIgnorecase {
			name = strings.ToLower(file.Name())
			word = strings.ToLower(e.searchWord)
		} else {
			name = file.Name()
			word = e.searchWord
		}
		if strings.Index(name, word) == -1 {
			continue
		}
		// get file times
		pathName := filepath.Join(path, file.Name())
		t, err := times.Stat(pathName)
		if err != nil {
			log.Printf("%s: %s\n", ErrGetTime, err)
			continue
		}

		access = t.AccessTime().Format(dateFmt)
		change = file.ModTime().Format(dateFmt)
		if t.HasBirthTime() {
			create = t.BirthTime().Format(dateFmt)
		}

		// get file permission, owner, group
		if stat, ok := file.Sys().(*syscall.Stat_t); ok {
			perm = file.Mode().String()

			uid := strconv.Itoa(int(stat.Uid))
			u, err := user.LookupId(uid)
			if err != nil {
				owner = uid
			} else {
				owner = u.Username
			}
			gid := strconv.Itoa(int(stat.Gid))
			g, err := user.LookupGroupId(gid)
			if err != nil {
				group = gid
			} else {
				group = g.Name
			}
		}

		// create entriey
		entries = append(entries, &Entry{
			Name:       file.Name(),
			Access:     access,
			Create:     create,
			Change:     change,
			Size:       file.Size(),
			Permission: perm,
			IsDir:      file.IsDir(),
			Owner:      owner,
			Group:      group,
			PathName:   pathName,
			Path:       path,
			Viewable:   true,
		})
	}

	e.entries = entries
	e.SetColumns()
	return entries
}
