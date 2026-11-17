package watch

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/znet"
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
	httpProxyClient = &http.Client{Timeout: 30 * time.Second}
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
		util.Log.Warnf("HTTP 端口 %d 不可用: %v，尝试自动分配端口\n", httpPort, pErr)
		port, pErr = getPort(0, true)
		if pErr != nil {
			util.Log.Errorf("无法启动 HTTP 服务: %v\n", pErr)
			return
		}
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

// changePayload builds the browser payload for files under the HTTP root.
// The event object is shared with the command runner, so the HTTP-relative
// name is written to a copy instead of mutating the original.
func changePayload(data *changedFile) ([]byte, bool) {
	if data == nil {
		return nil, false
	}
	root := strings.TrimSuffix(httpPath, "/")
	if root == "" || !strings.HasPrefix(data.Path, root+"/") {
		return nil, false
	}
	payload := *data
	payload.Name = strings.TrimPrefix(strings.TrimPrefix(data.Path, root), "/")
	msg, err := json.Marshal(&payload)
	if err != nil {
		return nil, false
	}
	return msg, true
}

func sendChang(data *changedFile) {
	if msg, ok := changePayload(data); ok {
		send(msg)
	}
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

// resolveStaticPath maps a request path onto httpPath. It rejects paths that
// would escape the served root (for example "/../../etc/passwd").
func resolveStaticPath(urlPath string) (string, bool) {
	urlPath = strings.ReplaceAll(urlPath, "\\", "/")
	clean := path.Clean("/" + urlPath)
	root := filepath.Clean(httpPath)
	pullPath := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, pullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(pullPath), true
}

func httpEntrance(c *znet.Context) {
	var err error
	method := c.Request.Method
	urlPath := c.Request.URL.Path

	c.SetHeader("cache-control", "no-store")
	if urlPath == "/" || urlPath == "" {
		urlPath = "/index.html"
	}

	pullPath, ok := resolveStaticPath(urlPath)
	if !ok {
		c.String(404, "file not found")
		return
	}
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

func injectingCode(file string) string {
	content, err := os.ReadFile(file)
	if err != nil {
		util.Log.Printf("%s\n", err)
		return ""
	}

	var js string
	switch httpType {
	case "web":
		js = webJs
	case "vue-spa":
		js = vueSpaJs
	case "vue-run":
		js = vueHotReload
	}
	if js == "" {
		return string(content)
	}

	return injectJavaScriptToHTML(string(content), js)
}

func proxy(_ string, w http.ResponseWriter, r *http.Request) (err error) {
	host, scheme := urlParse(httpProxy)
	if host == "" {
		err = errors.New("404")
		return
	}

	targetURL := &url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     r.URL.Path,
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

	client := httpProxyClient
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
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		htmlContent := string(body)
		injectedHTML := injectJavaScriptToHTML(htmlContent, `var zWatchAll=true;`+getInjectJS())

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

func injectJavaScriptToHTML(htmlContent, injectJS string) string {
	lowerHTML := strings.ToLower(htmlContent)

	if bodyIndex := strings.LastIndex(lowerHTML, "</body>"); bodyIndex != -1 {
		return htmlContent[:bodyIndex] + "<script>" + injectJS + "</script>" + htmlContent[bodyIndex:]
	}

	if htmlIndex := strings.LastIndex(lowerHTML, "</html>"); htmlIndex != -1 {
		return htmlContent[:htmlIndex] + "<script>" + injectJS + "</script>" + htmlContent[htmlIndex:]
	}

	return htmlContent + "<script>" + injectJS + "</script>"
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
	if !strings.Contains(httpProxy, "://") {
		httpProxy = "http://" + httpProxy
	}
	var host, scheme string
	p, err := url.Parse(httpProxy)
	if err != nil {
		log.Println(err)
	} else {
		host, scheme = p.Host, p.Scheme
	}
	return host, scheme
}
