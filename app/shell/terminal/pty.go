package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/creack/pty"
)

type PTY struct {
	cmd  *exec.Cmd
	ptmx *os.File
}

func NewPTY(shellPath string, rows, cols uint16) (*PTY, error) {
	if shellPath == "" {
		shellPath = getDefaultShell()
	}

	cmd := exec.Command(shellPath)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}); err != nil {
		ptmx.Close()
		return nil, fmt.Errorf("failed to set pty size: %w", err)
	}

	return &PTY{
		cmd:  cmd,
		ptmx: ptmx,
	}, nil
}

func (p *PTY) Read(b []byte) (int, error) {
	return p.ptmx.Read(b)
}

func (p *PTY) Write(b []byte) (int, error) {
	return p.ptmx.Write(b)
}

func (p *PTY) Resize(rows, cols uint16) error {
	return pty.Setsize(p.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

func (p *PTY) Close() error {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
	if p.ptmx != nil {
		return p.ptmx.Close()
	}
	return nil
}

func (p *PTY) Wait() error {
	if p.cmd != nil {
		return p.cmd.Wait()
	}
	return nil
}

func (p *PTY) Reader() io.Reader {
	return p.ptmx
}

func (p *PTY) Writer() io.Writer {
	return p.ptmx
}

func getDefaultShell() string {
	shell := os.Getenv("SHELL")
	if shell != "" {
		return shell
	}

	if runtime.GOOS == "windows" {
		if powershell := os.Getenv("COMSPEC"); powershell != "" {
			return powershell
		}
		return "cmd.exe"
	}

	return "/bin/sh"
}
