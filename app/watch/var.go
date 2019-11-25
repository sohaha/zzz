package watch

import (
	"github.com/sohaha/zzz/util"
	"path/filepath"

	"github.com/spf13/viper"
	"os"
	"sync"
)

const (
	// ErrCfgNotFoun config file does not exist
	ErrCfgNotFoun = "config file not found"
)

var (
	cfgPath            string
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
