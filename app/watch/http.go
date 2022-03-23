package watch

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/znet"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zlsgo/ztype"
	"gopkg.in/olahol/melody.v1"

	"github.com/sohaha/zzz/util"
)

var (
	httpType        string
	httpPort        int
	httpPath        string
	httpProxy       string
	httpRoot        string
	httpOpenBrowser bool
	httpCloseLocal  bool
	ws              *melody.Melody
	ignoreFormat    []string
)

func initHTTP() {
	httpType = v.GetString("http.type")
	httpProxy = v.GetString("http.proxy")
	httpRoot = v.GetString("http.root")
	httpPort = v.GetInt("http.port")
	httpOpenBrowser = v.GetBool("http.openBrowser")
	v.SetDefault("http.closeLocal", false)
	httpCloseLocal = v.GetBool("http.closeLocal")
	if httpType == "vue-run" {
		types := v.GetStringSlice("monitor.types")
		ignoreFormat = []string{".vue", ".css", ".html", ".js", ".es6"}
		types = append(types, ignoreFormat...)
		v.Set("monitor.types", types)
	}
}

func httpRun() {
	httpPath = filepath.ToSlash(filepath.Clean(projectFolder + "/" + httpRoot))
	port, pErr := getPort(httpPort, false)
	if pErr != nil {
		log.Fatalln(pErr)
	}
	httpPort = port
	// isSap = strings.ToLower(httpType) == "vue-spa"
	if httpType == "" || httpType == "none" {
		return
	}

	ws = melody.New()
	service := znet.New()
	// service.SetMode(znet.DebugMode)
	service.Log.ResetFlags(0)
	service.Log.SetPrefix("")
	service.NotFoundHandler(func(c *znet.Context) {
		httpEntrance(c)
		_ = c.PrevContent()
	})

	service.GET("/", func(c *znet.Context) {
		if c.IsWebsocket() {
			_ = ws.HandleRequest(c.Writer, c.Request)
		} else {
			httpEntrance(c)
		}
	})

	service.POST("/___VueRunMinifyApi___", util.MinifyHandle)

	ws.HandleMessage(func(s *melody.Session, data []byte) {
		// msg := string(data[:])
		// util.Log.Println(msg)
		_ = ws.Broadcast(data)
	})
	host := ":" + ztype.ToString(port)
	service.SetAddr(host)
	domain := "http://127.0.0.1" + host
	// util.Log.Printf("WebServe: %v", domain)
	if httpOpenBrowser {
		_ = openBrowser(domain)
	}
	znet.Run()
}

func sendChang(data *changedFile) {
	if !strings.HasPrefix(data.Path, httpPath) {
		return
	}
	relativePath := strings.TrimPrefix(data.Path, httpPath)
	data.Name = strings.TrimPrefix(relativePath, "/")
	_json, _ := json.Marshal(data)
	send(_json)
}

func send(msg []byte) {
	if ws != nil {
		go func() {
			if err := ws.Broadcast(msg); err != nil {
				util.Log.Println(err)
			}
		}()
	}
}

func httpEntrance(c *znet.Context) {
	var err error
	method := c.Request.Method
	urlPath := c.Request.URL.Path

	c.SetHeader("cache-control", "no-store")
	if urlPath == "/" || urlPath == "" {
		urlPath = "/index.html"
	}

	pullPath := httpPath + urlPath
	if !httpCloseLocal && zfile.FileExist(pullPath) {
		ext := path.Ext(pullPath)
		ext = strings.ToLower(ext)
		switch ext {
		case ".html":
			if method == "GET" {
				c.HTML(200, injectingCode(pullPath))
				return
			}
		}
		c.File(pullPath)
		return
	}
	if err = proxy(pullPath, c.Writer, c.Request); err != nil {
		c.String(404, "file not found")
	} else {
		c.Abort(200)
	}
}

func injectingCode(file string) (data string) {
	inputFile, inputError := os.Open(file)
	if inputError != nil {
		util.Log.Printf("%s\n", inputError)
		return
	}
	defer inputFile.Close()
	inputReader := bufio.NewReader(inputFile)
	isEnd := false
	isHTML := false
	var html = zstring.Buffer()
	for !isEnd {
		inputString, readerError := inputReader.ReadString('\n')
		if !isHTML && strings.ContainsAny(inputString, "</html>") {
			isHTML = true
		}
		html.WriteString(inputString)
		if readerError == io.EOF {
			isEnd = true
		}
	}
	html.WriteString("<script>")

	if httpType == "web" {
		// 插入自动刷新
		html.WriteString(webJs)
	} else if httpType == "vue-spa" {
		// 插入spa热更新
		html.WriteString(vueSpaJs)
	} else if httpType == "vue-run" {
		html.WriteString(vueHotReload)
		// html.WriteString(vueRunExport)
	}
	html.WriteString("</script>")
	data = html.String()
	return
}

func proxy(_ string, w http.ResponseWriter, r *http.Request) (err error) {
	host, scheme := urlParse(httpProxy)
	if host == "" {
		err = errors.New("404")
	} else {
		var ReverseProxy = httputil.ReverseProxy{
			ErrorLog: log.New(ioutil.Discard, "", 0),
			Director: func(req *http.Request) {
				req.URL.Scheme = scheme
				req.URL.Host = host
				req.Host = host
			},
		}
		ReverseProxy.ServeHTTP(w, r)
	}
	return
}

func urlParse(httpProxy string) (string, string) {
	var host, scheme string
	p, err := url.Parse(httpProxy)
	if err != nil {
		log.Println(err)
	} else {
		host, scheme = p.Host, p.Scheme
	}
	return host, scheme
}
