package bindata

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	
	"github.com/sohaha/zlsgo/zlog"
)

var mtimeDate = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

func RunStatic(src, dest, pkgTags, pkg string, noMtime, noCompress, force bool) string {
	file, err := generateSource(src, pkgTags, pkg, noMtime, noCompress)
	if err != nil {
		zlog.Fatal(err)
	}
	
	destDir := path.Join(dest, pkg)
	err = os.MkdirAll(destDir, 0755)
	if err != nil {
		zlog.Fatal(err)
	}
	
	err = rename(file.Name(), path.Join(destDir, nameSourceFile), force)
	if err != nil {
		zlog.Fatal(err)
	}
	return path.Join(destDir, nameSourceFile)
}

func generateSource(srcPath, pkgTags, pkg string, noMtime, noCompress bool) (file *os.File, err error) {
	var (
		buffer    bytes.Buffer
		zipWriter io.Writer
	)
	
	zipWriter = &buffer
	f, err := ioutil.TempFile("", pkg)
	if err != nil {
		return
	}
	
	zipWriter = io.MultiWriter(zipWriter, f)
	defer f.Close()
	
	w := zip.NewWriter(zipWriter)
	if err = filepath.Walk(srcPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() || strings.HasPrefix(fi.Name(), ".") {
			return nil
		}
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		fHeader, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		if noMtime {
			fHeader.Modified = mtimeDate
		}
		fHeader.Name = filepath.ToSlash(relPath)
		if !noCompress {
			fHeader.Method = zip.Deflate
		}
		f, err := w.CreateHeader(fHeader)
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return err
	}); err != nil {
		return
	}
	if err = w.Close(); err != nil {
		return
	}
	var qb bytes.Buffer
	
	if pkgTags != "" {
		_, _ = fmt.Fprintf(&qb, `%s`, "// +build "+pkgTags+"\n")
	}
	_, _ = fmt.Fprintf(&qb, `package %s
%s
const zipData = "`, pkg, importString)
	
	fprintZipData(&qb, buffer.Bytes())
	
	_, _ = fmt.Fprintf(&qb, `"%s`, functionStrings)
	
	if err = ioutil.WriteFile(f.Name(), qb.Bytes(), 0644); err != nil {
		return
	}
	return f, nil
}

func rename(src, dest string, flagForce bool) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	rc, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		rc.Close()
		_ = os.Remove(src)
	}()
	
	if _, err = os.Stat(dest); !os.IsNotExist(err) {
		if flagForce {
			if err = os.Remove(dest); err != nil {
				return fmt.Errorf("file %q could not be deleted", dest)
			}
		} else {
			return fmt.Errorf("file %q already exists; use -force to overwrite", dest)
		}
	}
	
	wc, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer wc.Close()
	
	if _, err = io.Copy(wc, rc); err != nil {
		_ = os.Remove(dest)
	}
	return err
}

func fprintZipData(dest *bytes.Buffer, zipData []byte) {
	for _, b := range zipData {
		if b == '\n' {
			dest.WriteString(`\n`)
			continue
		}
		if b == '\\' {
			dest.WriteString(`\\`)
			continue
		}
		if b == '"' {
			dest.WriteString(`\"`)
			continue
		}
		if (b >= 32 && b <= 126) || b == '\t' {
			dest.WriteByte(b)
			continue
		}
		_, _ = fmt.Fprintf(dest, "\\x%02x", b)
	}
}

func commentLines(lines string) string {
	lines = "// " + strings.Replace(lines, "\n", "\n// ", -1)
	return lines
}
