package stress

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	reggen "github.com/lucasjones/reggen"
	http2 "golang.org/x/net/http2"
)

// splits on delim into parts and trims whitespace
// delim1 splits the pairs, delim2 splits amongst the pairs
// like parseKeyValString("key1: val2, key3 : val4,key5:val6 ", ",", ":") becomes
// ["key1"]->"val2"
// ["key3"]->"val4"
// ["key5"]->"val6"
func parseKeyValString(keyValStr, delim1, delim2 string) (map[string]string, error) {
	m := make(map[string]string)
	if delim1 == delim2 {
		return m, errors.New("分隔符不能相同")
	}
	pairs := strings.SplitN(keyValStr, delim1, -1)
	for _, pair := range pairs {
		parts := strings.SplitN(pair, delim2, 2)
		if len(parts) != 2 {
			return m, errors.New("解析为两部分失败")
		}
		key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if key == "" || val == "" {
			return m, errors.New("键或值为空")
		}
		m[key] = val
	}
	return m, nil
}

// build the http request out of the target's config
func buildRequest(t Target) (http.Request, error) {
	if t.URL == "" {
		return http.Request{}, errors.New("URL 为空")
	}
	if len(t.URL) < 8 {
		return http.Request{}, errors.New("URL 过短")
	}
	//prepend "http://" if scheme not provided
	//maybe a cleaner way to do this via net.url?
	if t.URL[:7] != "http://" && t.URL[:8] != "https://" {
		t.URL = "http://" + t.URL
	}
	var urlStr string
	var err error
	//when regex set, generate urls
	if t.RegexURL {
		urlStr, err = reggen.Generate(t.URL, 10)
		if err != nil {
			return http.Request{}, errors.New("解析正则表达式失败: " + err.Error())
		}
	} else {
		urlStr = t.URL
	}
	URL, err := url.Parse(urlStr)
	if err != nil {
		return http.Request{}, errors.New("解析 URL 失败 " + urlStr + " : " + err.Error())
	}
	if URL.Host == "" {
		return http.Request{}, errors.New("主机名为空")
	}

	if t.DNSPrefetch {
		addrs, err := net.LookupHost(URL.Hostname())
		if err != nil {
			return http.Request{}, errors.New("预取主机失败 " + URL.Host)
		}
		if len(addrs) == 0 {
			return http.Request{}, errors.New("未找到地址 " + URL.Host)
		}
		URL.Host = addrs[0]
	}

	//setup the request
	var req *http.Request
	if t.BodyFilename != "" {
		fileContents, fileErr := ioutil.ReadFile(t.BodyFilename)
		if fileErr != nil {
			return http.Request{}, errors.New("读取文件内容失败 " + t.BodyFilename + ": " + fileErr.Error())
		}
		req, err = http.NewRequest(t.Method, URL.String(), bytes.NewBuffer(fileContents))
	} else if t.Body != "" {
		req, err = http.NewRequest(t.Method, URL.String(), bytes.NewBuffer([]byte(t.Body)))
	} else {
		req, err = http.NewRequest(t.Method, URL.String(), nil)
	}
	if err != nil {
		return http.Request{}, errors.New("创建请求失败: " + err.Error())
	}
	// add headers
	if t.Headers != "" {
		headerMap, err := parseKeyValString(t.Headers, ",", ":")
		if err != nil {
			return http.Request{}, errors.New("解析请求头失败: " + err.Error())
		}
		for key, val := range headerMap {
			req.Header.Add(key, val)
		}
	}

	req.Header.Set("User-Agent", t.UserAgent)

	// add cookies
	if t.Cookies != "" {
		cookieMap, err := parseKeyValString(t.Cookies, ";", "=")
		if err != nil {
			return http.Request{}, errors.New("解析 Cookie 失败: " + err.Error())
		}
		for key, val := range cookieMap {
			req.AddCookie(&http.Cookie{Name: key, Value: val})
		}
	}

	if t.BasicAuth != "" {
		authMap, err := parseKeyValString(t.BasicAuth, ",", ":")
		if err != nil {
			return http.Request{}, errors.New("解析基础认证失败: " + err.Error())
		}
		for key, val := range authMap {
			req.SetBasicAuth(key, val)
			break
		}
	}
	return *req, nil
}

func createClient(target Target) *http.Client {
	tr := &http.Transport{}
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: !target.EnforceSSL}
	tr.DisableCompression = !target.Compress
	tr.DisableKeepAlives = !target.KeepAlive
	if target.NoHTTP2 {
		tr.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	} else {
		_ = http2.ConfigureTransport(tr)
	}
	var timeout time.Duration
	if target.Timeout != "" {
		timeout, _ = time.ParseDuration(target.Timeout)
	} else {
		timeout = time.Duration(0)
	}
	client := &http.Client{Timeout: timeout, Transport: tr}
	if !target.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client
}
