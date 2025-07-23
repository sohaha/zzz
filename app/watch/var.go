package watch

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/sohaha/zlsgo/zfile"

	"github.com/sohaha/zzz/util"

	"github.com/spf13/viper"
)

const (
	// ErrCfgNotFoun config file does not exist
	ErrCfgNotFoun = "config file not found"
)

var (
	projectFolder      = "."
	watcher            FileWatcher
	watchDirs          = make([]string, 0)
	exceptDirs         = make([]string, 0)
	types              = make([]string, 0)
	includeDirs        = make([]string, 0)
	globalErr          error
	done               = make(chan bool, 1)
	signalChan         = make(chan os.Signal, 1)
	lastPid            int
	task               *taskType
	execCommand        []string
	execFileExt        []string
	startupExecCommand []string
	startup            bool
	pushTimer          sync.Map
	fileDebouncer      *debouncer
	pendingFiles       sync.Map
	v                  *viper.Viper
	ignoreDirectory    = [...]string{".git", ".vscode", ".svn", ".idea", ".github"}
	ignoreFormat       []string
)

type changedFile struct {
	Name    string
	Path    string
	Ext     string
	Type    string
	Changed int64
}

func init() {
	v = viper.New()
	if projectFolder, globalErr = os.Getwd(); globalErr != nil {
		util.Log.Fatal(globalErr)
	}
	projectFolder = zfile.RealPath(projectFolder)
	projectFolder = filepath.ToSlash(projectFolder)
	projectFolder, _ = filepath.Abs(projectFolder)
}
