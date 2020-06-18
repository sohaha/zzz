package watch

import (
	"errors"
	"io/ioutil"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/ztype"
)

func dirParse2Array(s string) []string {
	a := strings.Split(s, ",")
	r := make([]string, 0)

	for i := 0; i < len(a); i++ {
		if ss := strings.Trim(a[i], " "); ss != "" {
			r = append(r, ss)
		}
	}
	return r
}

func isIgnoreDirectory(folder string) bool {
	base := filepath.Base(folder)
	for _, v := range ignoreDirectory {
		if base == v {
			return true
		}
	}
	return false
}

func listFile(folder string, fun func(string)) {
	if isIgnoreDirectory(folder) {
		return
	}

	files, _ := ioutil.ReadDir(folder)
	for _, file := range files {
		if file.IsDir() {
			d := folder + "/" + file.Name()
			fun(d)
			listFile(d, fun)
		}
	}
}

func arrayUniqueAdd(a []string, add string) []string {
	if inStringArray(add, a) {
		return a
	}
	return append(a, add)
}

func arrayRemoveElement(a []string, r string) []string {
	i := -1
	for k, v := range a {
		if v == r {
			i = k
			break
		}
	}
	if i == -1 {
		return a
	}
	if len(a) == 1 && i == 0 {
		return []string{}
	}
	return append(a[:i], a[i+1:]...)
}

func inStringArray(value string, arr []string) bool {
	for _, v := range arr {
		if value == v {
			return true
		}
	}
	return false
}

func cmdParse2Array(s string, cf *changedFile) []string {
	a := strings.Split(s, " ")
	r := make([]string, 0)
	for i := 0; i < len(a); i++ {
		if ss := strings.Trim(a[i], " "); ss != "" {
			r = append(r, strParseRealStr(ss, cf))
		}
	}
	return r
}

func strParseRealStr(s string, cf *changedFile) string {
	return strings.Replace(
		strings.Replace(
			strings.Replace(s, "{{file}}", cf.Name, -1),
			"{{ext}}", cf.Ext, -1,
		),
		"{{changed}}", strconv.FormatInt(cf.Changed, 10), -1,
	)
}

func getDelay() time.Duration {
	delay := task.delay
	if delay <= 0 {
		delay = 100
	}
	return time.Millisecond * time.Duration(delay)
}

func getPort(port int, generate bool) (newPort int, err error) {
	host := "127.0.0.1:" + ztype.ToString(port)
	listener, err := net.Listen("tcp", host)
	if err != nil {
		if !generate && port != 0 {
			return 0, err
		}
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, err
		}
	}
	defer listener.Close()
	addr := listener.Addr().String()
	_, portString, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(portString)
}

func openBrowser(url string) error {
	var err error
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = errors.New("unsupported platform")
	}
	return err
}
