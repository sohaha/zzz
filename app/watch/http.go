package watch

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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
	html := zstring.Buffer()
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

	switch httpType {
	case "web":
		html.WriteString(webJs)
	case "vue-spa":
		html.WriteString(vueSpaJs)
	case "vue-run":
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
		return
	}

	targetURL := &url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		return err
	}

	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	proxyReq.Host = host

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	contentType := resp.Header.Get("Content-Type")
	shouldInject := httpType == "web" && strings.Contains(strings.ToLower(contentType), "text/html")

	if shouldInject {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		htmlContent := string(body)
		injectedHTML := injectJavaScriptToHTML(htmlContent)
		
		w.Header().Del("Content-Length")
		
		w.WriteHeader(resp.StatusCode)
		
		_, err = w.Write([]byte(injectedHTML))
		return err
	} else {
		w.WriteHeader(resp.StatusCode)
		
		_, err = io.Copy(w, resp.Body)
		return err
	}
}

func injectJavaScriptToHTML(htmlContent string) string {
	lowerHTML := strings.ToLower(htmlContent)
	
	if bodyIndex := strings.LastIndex(lowerHTML, "</body>"); bodyIndex != -1 {
		return htmlContent[:bodyIndex] + "<script>" + getInjectJS() + "</script>" + htmlContent[bodyIndex:]
	}
	
	if htmlIndex := strings.LastIndex(lowerHTML, "</html>"); htmlIndex != -1 {
		return htmlContent[:htmlIndex] + "<script>" + getInjectJS() + "</script>" + htmlContent[htmlIndex:]
	}
	
	return htmlContent + "<script>" + getInjectJS() + "</script>"
}

func getInjectJS() string {
	switch httpType {
	case "web":
		return webJs
	case "vue-spa":
		return vueSpaJs
	case "vue-run":
		return vueHotReload
	default:
		return webJs
	}
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
