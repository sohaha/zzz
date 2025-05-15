package watch

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sohaha/zlsgo/zlog"

	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zlsgo/ztype"
	"github.com/sohaha/zlsgo/zutil"

	"github.com/sohaha/zzz/util"
)

type cmdType struct {
	cmd     *exec.Cmd
	putLock sync.Mutex
	runLock sync.Mutex
}

type taskType struct {
	cmd        *exec.Cmd
	cmdExt     map[string]*cmdType
	lastTaskID int64
	delay      int
	putLock    sync.Mutex
	runLock    sync.Mutex
}

func initTask() {
	task = &taskType{
		delay:  v.GetInt("other.delayMillSecond"),
		cmdExt: make(map[string]*cmdType),
	}
	execCommand = v.GetStringSlice("command.exec")
	startupExecCommand = v.GetStringSlice("command.startupExec")
	startup = v.GetBool("command.startup")
}

func (t *taskType) Put(cf *changedFile) {
	if t.delay < 1 {
		t.preRun(cf)
		return
	}
	t.putLock.Lock()
	defer t.putLock.Unlock()
	t.lastTaskID = cf.Changed
	go func() {
		<-time.Tick(time.Millisecond * time.Duration(t.delay))
		if t.lastTaskID > cf.Changed {
			return
		}
		t.preRun(cf)
	}()
}

func (t *taskType) preRun(cf *changedFile) {
	cloes(t.cmd)
	fileExt := zstring.Ucfirst(strings.TrimPrefix(cf.Ext, "."))
	if fileExt != "" {
		extCommand := v.GetStringSlice("command.exec" + fileExt)
		if cmd, ok := t.cmdExt[fileExt]; ok {
			cloes(cmd.cmd)
		}
		if !isIgnoreType(fileExt) {
			go t.run(cf, execCommand, true)
		}
		go t.run(cf, extCommand, true, fileExt)
	} else {
		go t.run(cf, execCommand, true)
		if cf.Path == "" {
			for _, fileExt := range execFileExt {
				extCommand := v.GetStringSlice("command.exec" + fileExt)
				go t.run(cf, extCommand, true, fileExt)
			}
		}
	}
}

func (t *taskType) run(cf *changedFile, commands []string, outpuContent bool, ext ...string) *taskType {
	var (
		logPrefix string
		fileExt   string
		hasExtCmd bool
	)
	if len(ext) > 0 {
		fileExt = ext[0]
		if extCmd, ok := t.cmdExt[fileExt]; ok {
			hasExtCmd = true
			extCmd.runLock.Lock()
			defer func() {
				extCmd.runLock.Unlock()
			}()
		}
	} else {
		t.runLock.Lock()
		defer func() {
			t.runLock.Unlock()
		}()
	}
	defer func() {
		if r := recover(); r != nil {
			util.Log.Fatal(r)
		}
	}()
	l := len(commands)
	if l <= 0 {
		// util.Log.Println("no command")
		return nil
	}
	for i := 0; i < l; i++ {
		c := util.OSCommand(commands[i])
		if c == "" {
			// util.Log.Println("Ignore command:",commands[i])
			continue
		}
		carr := cmdParse2Array(c, cf)
		if outpuContent {
			util.Log.Printf("Command: %v\n", carr)
		} else {
			util.Log.Printf("Background command: %v\n", carr)
			continue
		}

		cmd := command(fixCmd(carr))
		if fileExt == "" {
			t.cmd = cmd
			logPrefix = strings.Repeat(" ", 2)
		} else {
			if hasExtCmd {
				t.cmdExt[fileExt].cmd = cmd
			} else {
				t.cmdExt[fileExt] = &cmdType{cmd: cmd}
			}
			logPrefixBuffer := zstring.Buffer()
			logPrefixBuffer.WriteString("  ")
			logPrefixBuffer.WriteString("[")
			logPrefixBuffer.WriteString(fileExt)
			logPrefixBuffer.WriteString("] ")
			logPrefix = util.Log.ColorTextWrap(zlog.ColorCyan, logPrefixBuffer.String())
		}
		stdout, err := cmd.StdoutPipe()
		stderr, stderrErr := cmd.StderrPipe()
		if err != nil {
			util.Log.Println("Error: ", err.Error())
			return nil
		}
		if stderrErr != nil {
			util.Log.Println("Error: ", stderrErr.Error())
			return nil
		}
		err = cmd.Start()
		if err != nil {
			util.Log.Println("command Error: ", err)
			break
		}

		ch := make(chan bool)
		show := func(line string) {
			prefix := fmt.Sprintf("%s%s", logPrefix, line)
			fmt.Print(prefix)
		}
		exportStd := func(stdout io.Reader) bool {
			reader := bufio.NewReader(stdout)
			for {
				line, err2 := reader.ReadString('\n')
				if err2 != nil {
					if io.EOF == err2 {
						line = strings.Replace(line, " ", "", -1)
						if line != "" {
							show(line + "\n")
						}
					}
					return true
				}

				if strings.Contains(line, "exit status 2") {
					return true
				}
				show(line)
			}
		}
		lastPid = cmd.Process.Pid
		if outpuContent {
			go func(stdout io.Reader) {
				ch <- exportStd(stdout)
			}(stdout)

			exportStd(stderr)
			waiting := func() {
				for ii := 1; ii <= 1; ii++ {
					<-ch
				}
			}
			if err = cmd.Wait(); err != nil {
				errMsg := err.Error()
				if !strings.Contains(errMsg, "exit status 1") && !strings.Contains(errMsg, "signal: killed") {
					util.Log.Println("command End:", err)
				}
				//  todo 其中一个命令报错后面的都不执行
				waiting()
				break
			} else {
				waiting()
				if cmd.Process != nil {
					if err = cmd.Process.Kill(); err != nil && (!strings.Contains(err.Error(), "os: process already finished")) {
						if cmd.ProcessState.String() != "exit status 0" {
							util.Log.Println("cmd cannot kill ", err)
						}
					}
				}
			}

		}
	}

	return t
}

func (t *taskType) runBackground(cf *changedFile, commands []string) []*exec.Cmd {
	l := len(commands)
	if l <= 0 {
		return nil
	}
	var r []*exec.Cmd

	for i := 0; i < l; i++ {
		carr := []string{strings.Join(cmdParse2Array(commands[i], cf), " ")}
		util.Log.Printf("Background command: %v\n", carr)
		cmd := command(fixCmd(carr))

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			util.Log.Error(err)
			continue
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			util.Log.Error(err)
			continue
		}

		err = cmd.Start()
		if err != nil {
			util.Log.Error(err)
		} else {
			r = append(r, cmd)
			go func(stdout, stderr io.ReadCloser) {
				go func() {
					scanner := bufio.NewScanner(stdout)
					for scanner.Scan() {
						util.Log.Printf("[BG] %s\n", scanner.Text())
					}
				}()

				go func() {
					scanner := bufio.NewScanner(stderr)
					for scanner.Scan() {
						util.Log.Printf("[BG] %s\n", scanner.Text())
					}
				}()
			}(stdout, stderr)
		}
	}
	return r
}

func cloes(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		if !zutil.IsWin() {
			p, e := os.FindProcess(-cmd.Process.Pid)
			if e == nil {
				_ = p.Signal(syscall.SIGINT)
			}
		} else {
			cmd := exec.Command("TASKKILL", "/T", "/F", "/PID", ztype.ToString(cmd.Process.Pid))
			_, _ = cmd.CombinedOutput()
		}
		time.Sleep(time.Second / 6)
		_ = cmd.Process.Kill()
	}
}

func command(carr []string) *exec.Cmd {
	cmd := exec.Command(carr[0], carr[1:]...)
	sCmd(cmd)
	cmd.Dir = projectFolder
	cmd.Env = os.Environ()
	return cmd
}

func fixCmd(carr []string) []string {
	carr = []string{strings.Join(carr, " ")}
	if zutil.IsWin() {
		carr = append([]string{"cmd", "/C"}, carr...)
	} else {
		carr = append([]string{"sh", "-c"}, carr...)
	}
	return carr
}
