package watch

import (
	"os"
	"path/filepath"
	"sync"

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
	globalErr          error
	done               = make(chan bool, 1)
	signalChan         = make(chan os.Signal, 1)
	lastPid            int
	task               *taskType
	execCommand        []string
	startupExecCommand []string
	startup            bool
	pushTimer          sync.Map
	v                  *viper.Viper
	ignoreDirectory    = [...]string{".git", ".vscode", ".svn", ".idea"}
	// waitGroup     sync.WaitGroup
)

type changedFile struct {
	Name    string
	Path    string
	Changed int64
	Ext     string
	Type    string
}

func init() {
	v = viper.New()
	if projectFolder, globalErr = os.Getwd(); globalErr != nil {
		util.Log.Fatal(globalErr)
	}
	projectFolder = filepath.ToSlash(projectFolder)
	projectFolder, _ = filepath.Abs(projectFolder)
}
