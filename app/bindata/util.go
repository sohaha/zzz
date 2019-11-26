package bindata

const (
	nameSourceFile = "static.go"
	importString   = `
import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"github.com/sohaha/zlsgo/znet"
	"github.com/sohaha/zlsgo/zstring"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)
`
	functionStrings = `

type file struct {
	os.FileInfo
	data []byte
	fs   *binFs
}

type binFs struct {
	files map[string]file
	dirs  map[string][]string
}
const indexHtml = "index.html"

// New Static returns a middleware handler that serves static files in the given directory
func New(urlPrefix string) (func(c *znet.Context) error, error) {
	fs, err := create(urlPrefix)
	if err != nil {
		return func(c *znet.Context) error {
			return err
		}, err
	}
	fileserver := http.FileServer(fs)
	// if urlPrefix != "" {
	// 	fileserver = http.StripPrefix(urlPrefix, fileserver)
	// }
	return func(c *znet.Context) error {
		urlPath := strings.TrimSpace(c.Request.URL.Path)
		
		if urlPath == urlPrefix || strings.HasSuffix(urlPath, "/") {
			urlPath = path.Join(urlPrefix, indexHtml)
		}
		
		if !strings.HasPrefix(urlPath, urlPrefix) {
			return errors.New("Path mismatch")
		}
		
		if f, err := fs.Open(urlPath); err == nil {
			fi, err := f.Stat()
			if err != nil || !fi.IsDir() {
				if strings.HasSuffix(urlPath, ".html") {
					c.SetHeader("Cache-Control", "no-cache")
				}
				c.Info.Code = 200
				fileserver.ServeHTTP(c.Writer, c.Request)
				c.Abort()
				return nil
			}
		}
			return errors.New("File does not exist")
	}, nil
}

func create(prefix string) (http.FileSystem, error) {
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	zipReader, err := zip.NewReader(strings.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}
	files := make(map[string]file, len(zipReader.File))
	dirs := make(map[string][]string)
	fs := &binFs{files: files, dirs: dirs}
	for _, zipFile := range zipReader.File {
		fi := zipFile.FileInfo()
		f := file{FileInfo: fi, fs: fs}
		f.data, err = unzip(zipFile)
		if err != nil {
			return nil, fmt.Errorf("error unzipping file %q: %s", zipFile.Name, err)
		}
		filename := zstring.Buffer()
		filename.WriteString(prefix)
		filename.WriteString(zipFile.Name)
		files[filename.String()] = f
	}
	for fn := range files {
		for dn := path.Dir(fn); dn != fn; {
			if _, ok := files[dn]; !ok {
				files[dn] = file{FileInfo: dirInfo{dn}, fs: fs}
			} else {
				break
			}
			fn, dn = dn, path.Dir(dn)
		}
	}
	for fn := range files {
		dn := path.Dir(fn)
		if fn != dn {
			fs.dirs[dn] = append(fs.dirs[dn], path.Base(fn))
		}
	}
	for _, s := range fs.dirs {
		sort.Strings(s)
	}
	return fs, nil
}

var _ = os.FileInfo(dirInfo{})

type dirInfo struct {
	name string
}

func (di dirInfo) Name() string       { return path.Base(di.name) }
func (di dirInfo) Size() int64        { return 0 }
func (di dirInfo) Mode() os.FileMode  { return 0755 | os.ModeDir }
func (di dirInfo) ModTime() time.Time { return time.Time{} }
func (di dirInfo) IsDir() bool        { return true }
func (di dirInfo) Sys() interface{}   { return nil }
func unzip(zf *zip.File) ([]byte, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return ioutil.ReadAll(rc)
}

// Open returns a file matching the given file name
func (fs *binFs) Open(name string) (http.File, error) {
	name = strings.Replace(name, "//", "/", -1)
	if f, ok := fs.files[name]; ok {
		return newHTTPFile(f), nil
	}
	return nil, os.ErrNotExist
}

func newHTTPFile(file file) *httpFile {
	if file.IsDir() {
		return &httpFile{file: file, isDir: true}
	}
	return &httpFile{file: file, reader: bytes.NewReader(file.data)}
}

type httpFile struct {
	file
	reader *bytes.Reader
	isDir  bool
	dirIdx int
}

// Read reads bytes into p, returns the number of read bytes
func (f *httpFile) Read(p []byte) (n int, err error) {
	if f.reader == nil && f.isDir {
		return 0, io.EOF
	}
	return f.reader.Read(p)
}

// Seek seeks to the offset
func (f *httpFile) Seek(offset int64, whence int) (ret int64, err error) {
	return f.reader.Seek(offset, whence)
}

// Stat stats the file
func (f *httpFile) Stat() (os.FileInfo, error) {
	return f, nil
}

// IsDir returns true if the file location represents a directory
func (f *httpFile) IsDir() bool {
	return f.isDir
}

// Readdir returns an empty slice of files
func (f *httpFile) Readdir(count int) ([]os.FileInfo, error) {
	var fis []os.FileInfo
	if !f.isDir {
		return fis, nil
	}
	di, ok := f.FileInfo.(dirInfo)
	if !ok {
		return nil, fmt.Errorf("failed to read directory: %q", f.Name())
	}
	fnames := f.file.fs.dirs[di.name]
	flen := len(fnames)
	start := f.dirIdx
	if start >= flen && count > 0 {
		return fis, io.EOF
	}
	var end int
	if count < 0 {
		end = flen
	} else {
		end = start + count
	}
	if end > flen {
		end = flen
	}
	for i := start; i < end; i++ {
		fis = append(fis, f.file.fs.files[path.Join(di.name, fnames[i])].FileInfo)
	}
	f.dirIdx += len(fis)
	return fis, nil
}

func (f *httpFile) Close() error {
	return nil
}
`
)
