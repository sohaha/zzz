package watch

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/mitchellh/go-homedir"
	"github.com/sohaha/zlsgo/zfile"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sohaha/zzz/util"
)

func StartCmd(watchCmd *cobra.Command) (app *cobra.Command) {
	app = &cobra.Command{
		Use:   "start",
		Short: "Start listening service",
		Run: func(cmd *cobra.Command, args []string) {
			var cfgPath string
			cfgPath, _ = cmd.Flags().GetString("cfg")
			if cfgPath == "" {
				cfgPath, _ = cmd.Parent().Flags().GetString("cfg")
			}
			if !zfile.FileExist(cfgPath) {
				oldCfg := "./zls-watch.yaml"
				if !zfile.FileExist(oldCfg) {
					homePath, homePathErr := homedir.Dir()
					if homePathErr == nil {
						v.AddConfigPath(homePath)
						v.SetConfigName("zzz-watch")
						v.SetConfigName("zls-watch")
					}
				} else {
					cfgPath = oldCfg
				}
			}
			// fmt.Println(cfgPath)
			if cfgPath != "" {
				v.SetConfigFile(cfgPath)
			}
			// cfgPath = zfile.RealPath(cfgPath)
			run(cmd)
		},
	}
	util.SetLimit(999999)
	watchCmd.AddCommand(app)
	return
}

func showInitCmd(cmd *cobra.Command) {
	util.Log.Infof(
		"Please run `%s %s %s` %s\n", cmd.Root().Use,
		cmd.Parent().Use, initCmdUse,
		"Generate configuration file",
	)
}

// run run watch cmd
func run(cmd *cobra.Command) {
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			util.Log.Error(ErrCfgNotFoun)
			showInitCmd(cmd)
		} else {
			util.Log.Error(err)
			showInitCmd(cmd)
		}

		return
	}
	// v.WatchConfig()
	// v.OnConfigChange(func(e fsnotify.Event) {
	// 	util.Log.Println("Config file changed:", e.Name)
	// 	signalChan <- os.Kill
	// })
	start()
}

func start() {
	initHTTP()
	initTask()
	var (
		err  error
		cmds []*exec.Cmd
	)
	poll := v.GetBool("monitor.poll")
	// watcher, err = fsnotify.NewWatcher()
	if poll {
		watcher = NewPollingWatcher()
	} else {
		watcher, err = NewWatcher()
	}
	if err != nil {
		util.Log.Fatal(err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events():
				if !ok {
					return
				}
				eventDispatcher(event)
			case err, ok := <-watcher.Errors():
				if !ok {
					return
				}
				util.Log.Println("error:", err)
			}

		}
	}()
	addWatcher()
	l := len(startupExecCommand)
	if l > 0 {
		cmds = task.runBackground(new(changedFile), startupExecCommand)
	}
	go func() {
		<-signalChan
		if cmds != nil {
			for _, v := range cmds {
				cloes(v)
			}
		}
		for _, v := range task.cmdExt {
			cloes(v.cmd)
		}
		cloes(task.cmd)

		if lastPid > 0 {
			p, e := os.FindProcess(-lastPid)
			if e == nil {
				_ = p.Signal(syscall.SIGINT)
				// cloes(lastPid)
			}
		}
		done <- true
	}()
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)

	if startup {
		task.preRun(new(changedFile))
	}

	httpRun()
	<-done
}
