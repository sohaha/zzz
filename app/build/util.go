package build

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zlsgo/ztime"
	"github.com/sohaha/zlsgo/ztype"
	"github.com/sohaha/zlsgo/zutil"
)

func GetGoVersion() string {
	cmd := exec.Command("go", "version")
	if out, err := cmd.CombinedOutput(); err == nil {
		goversion := strings.TrimPrefix(strings.TrimSpace(string(out)), "go version ")
		return goversion
	}
	return "None"
}

func GetBuildGitID() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err == nil {
		commitid := strings.TrimSpace(string(out))
		return commitid
	}
	return "None"
}

func GetBuildTime() string {
	return ztime.FormatTime(time.Now())
}

func DisabledCGO() bool {
	cgo := zutil.Getenv("CGO_ENABLED")

	if cgo == "" {
		_, s, _, _ := zshell.Run("go env CGO_ENABLED")
		cgo = zstring.TrimSpace(s)
	}

	return !ztype.ToBool(cgo)
}

func CheckGarble() error {
	envs := zshell.Env
	defer func() {
		zshell.Env = envs
	}()
	if code, _, _, err := zshell.Run("garble version"); err != nil || code != 0 {
		return errors.New("please install garble: go install mvdan.cc/garble@latest")
	}
	return nil
}

func CheckUPX() error {
	envs := zshell.Env
	defer func() {
		zshell.Env = envs
	}()

	if code, _, _, err := zshell.Run("upx --version"); err != nil || code != 0 {
		return errors.New("please install upx")
	}
	return nil
}

func RunUPX(file string, level string) error {
	envs := zshell.Env
	defer func() {
		zshell.Env = envs
	}()

	if !strings.HasPrefix(level, "-") {
		level = "-" + level
	}

	if level == "-0" {
		level = "--lzma"
	}

	code, _, _, _ := zshell.OutRun("upx "+level+" "+file, nil, os.Stdout, os.Stderr)
	if code != 0 {
		return errors.New("failed to compress with UPX")
	}
	return nil
}

func StripUPXHeaders(filePath string) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.New("failed to read file: " + err.Error())
	}
	header := [][]byte{
		{0x49, 0x6e, 0x66, 0x6f, 0x3a, 0x20, 0x54, 0x68, 0x69, 0x73},
		{0x20, 0x66, 0x69, 0x6c, 0x65, 0x20, 0x69, 0x73, 0x20, 0x70},
		{0x61, 0x63, 0x6b, 0x65, 0x64, 0x20, 0x77, 0x69, 0x74, 0x68},
		{0x20, 0x74, 0x68, 0x65, 0x20, 0x55, 0x50, 0x58, 0x20, 0x65},
		{0x78, 0x65, 0x63, 0x75, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x20},
		{0x70, 0x61, 0x63, 0x6b, 0x65, 0x72, 0x20, 0x68, 0x74, 0x74},
		{0x70, 0x3a, 0x2f, 0x2f, 0x75, 0x70, 0x78, 0x2e, 0x73, 0x66},
		{0x2e, 0x6e, 0x65, 0x74, 0x20, 0x24, 0x0a, 0x00, 0x24, 0x49},
		{0x64, 0x3a, 0x20, 0x55, 0x50, 0x58, 0x20, 0x33, 0x2e, 0x39},
		{0x36, 0x20, 0x43, 0x6f, 0x70, 0x79, 0x72, 0x69, 0x67, 0x68},
		{0x74, 0x20, 0x28, 0x43, 0x29, 0x20, 0x31, 0x39, 0x39, 0x36},
		{0x2d, 0x32, 0x30, 0x32, 0x30, 0x20, 0x74, 0x68, 0x65, 0x20},
		{0x55, 0x50, 0x58, 0x20, 0x54, 0x65, 0x61, 0x6d, 0x2e, 0x20},
		{0x41, 0x6c, 0x6c, 0x20, 0x52, 0x69, 0x67, 0x68, 0x74, 0x73},
		{0x20, 0x52, 0x65, 0x73, 0x65, 0x72, 0x76, 0x65, 0x64, 0x2e},
		{0x55, 0x50, 0x58, 0x21},
		{0x55, 0x50, 0x58, 0x30, 0x00},
		{0x55, 0x50, 0x58, 0x31, 0x00},
		{0x55, 0x50, 0x58, 0x32, 0x00},
	}

	isBuild64 := is64Bit(data)
	if isBuild64 {
		patchBytes(data, []byte{0x53, 0x56, 0x57, 0x55}, []byte{0x53, 0x57, 0x56, 0x55})
	} else {
		patchBytes(data, []byte{0x00, 0x60, 0xBE}, []byte{0x00, 0x55, 0xBE})
	}

	for _, v := range header {
		randomString := make([]byte, 5)
		_, err := rand.Read(randomString)
		if err != nil {
			return err
		}
		patchBytes(data, v, randomString)
	}

	err = ioutil.WriteFile(filePath, data, 0o644)
	return err
}

func is64Bit(data []byte) bool {
	peHeaderOffset := binary.LittleEndian.Uint32(data[0x3C:])
	machineType := binary.LittleEndian.Uint16(data[peHeaderOffset+4:])
	return machineType == 0x8664
}

func patchBytes(data []byte, oldBytes, newBytes []byte) {
	index := bytes.Index(data, oldBytes)
	if index != -1 {
		copy(data[index:index+len(newBytes)], newBytes)
	}
}
