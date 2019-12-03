package stress

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zzz/util"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	prefixSpace = "    "

	httpsTemplate = `` +
		prefixSpace + `  DNS Lookup   TCP Connection   TLS Handshake   Server Processing   Content Transfer` + "\n" +
		prefixSpace + ` %s  |     %s  |    %s  |        %s  |       %s  |` + "\n" +
		prefixSpace + `            |                |               |                   |                  |` + "\n" +
		prefixSpace + `   namelookup:%s      |               |                   |                  |` + "\n" +
		prefixSpace + `                       connect:%s     |                   |                  |` + "\n" +
		prefixSpace + `                                   pretransfer:%s         |                  |` + "\n" +
		prefixSpace + `                                                     starttransfer:%s        |` + "\n" +
		prefixSpace + `                                                                                total:%s`

	httpTemplate = `` +
		prefixSpace + `  DNS Lookup   TCP Connection   Server Processing   Content Transfer` + "\n" +
		prefixSpace + ` %s  |     %s  |        %s  |       %s  |` + "\n" +
		prefixSpace + `            |                |                   |                  |` + "\n" +
		prefixSpace + `   namelookup:%s      |                   |                  |` + "\n" +
		prefixSpace + `                       connect:%s         |                  |` + "\n" +
		prefixSpace + `                                     starttransfer:%s        |` + "\n" +
		prefixSpace + `                                                                total:%s`
)

var (
	TotalRequest        int     // 总请求数量
	FailedTransactions  int     // 请求失败的次数
	ErrorTransactions   int     // 请求异常的次数
	SuccessRate         float64 // 成功率
	TransactionRate     float64 // 平均每秒处理请求数 = 总请求次数/总耗时
	ElapsedTime         float64 // 总耗时
	LongestTransaction  float64 // 最长耗时
	ShortestTransaction float64 // 最短耗时
	waitReq             sync.WaitGroup
	waitStats           sync.WaitGroup
	timeout             time.Duration
	duration            time.Duration
	concurrency         uint
	showData            bool
	showStat            bool
	hideLog             bool
	tooManyOpenFiles    bool
	transport           = &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		MaxConnsPerHost:   0,
		DisableKeepAlives: false,
		DialContext: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		// Proxy:           proxy,
	}
)

type Cli struct {
	Timeout     *int
	Duration    *int
	RequestUrl  *string
	Concurrency *int
	Debug       *bool
	Stat        *bool
	Header      *string
	Body        *string
	Method      *string
	Hidelog     *bool
}

type Signal struct {
	MaxMinValue        chan float64
	FailedTransactions chan bool
	ErrorTransactions  chan bool
	TotalRequest       chan bool
}

type RequestContext struct {
	RawUrl        string
	Method        string
	Body          string
	HeaderKVSlice [][2]string
}

func httpSendRequest(reqData *RequestContext, signal Signal) {
	defer waitReq.Done()
	httpError := false
	// proxy := func(_ *http.Request) (*url.URL, error) {
	//      return url.Parse("http://127.0.0.1:80")
	//  }
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	req, err := http.NewRequest(reqData.Method, reqData.RawUrl, strings.NewReader(reqData.Body))
	if err != nil {
		util.Log.Println(util.Log.ColorTextWrap(zlog.ColorRed, "Request ERROR: "+err.Error()))
		return
	}
	var t0, t1, t2, t3, t4, t5, t6, t7 time.Time
	connectedInfo := zstring.Buffer()

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) { t0 = time.Now() },
		DNSDone:  func(_ httptrace.DNSDoneInfo) { t1 = time.Now() },
		ConnectStart: func(_, _ string) {
			if t1.IsZero() {
				// connecting to IP
				t1 = time.Now()
			}
		},
		ConnectDone: func(net, addr string, err error) {
			if err != nil {
				// httpError = true
				// util.Log.Errorf("unable to connect to host %v: %v", addr, err)
				return
			}
			t2 = time.Now()
			if showStat {
				connectedInfo.WriteString("      Connected: ")
				connectedInfo.WriteString(addr)
				connectedInfo.WriteString("\n")
			}
		},
		GotConn:              func(_ httptrace.GotConnInfo) { t3 = time.Now() },
		GotFirstResponseByte: func() { t4 = time.Now() },
		TLSHandshakeStart:    func() { t5 = time.Now() },
		TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { t6 = time.Now() },
	}
	req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))
	if reqData.Method != "GET" {
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	}

	req.Header.Set("Connection", "close")
	for _, v := range reqData.HeaderKVSlice {
		req.Header.Add(v[0], v[1])
	}
	var diffTime float64
	request := func() (*http.Response, error) {
		// startTime := time.Now()
		resp, err := client.Do(req)
		t7 = time.Now()
		// endTime := time.Now()
		// diffTime = endTime.Sub(startTime).Seconds()
		return resp, err
	}
	resp, err := request()
	logsData := zstring.Buffer()
	if err != nil {
		httpError = true
		errMsg := err.Error()
		if !strings.Contains(errMsg, "too many open files") {
			signal.TotalRequest <- true
			signal.FailedTransactions <- true
			if strings.Contains(errMsg, "Timeout") {
				logsData.WriteString(" ")
				logsData.WriteString(util.Log.ColorTextWrap(zlog.ColorRed, strconv.Itoa(http.StatusGatewayTimeout)))
			} else {
				signal.ErrorTransactions <- true
				util.Log.Println(util.Log.ColorTextWrap(zlog.ColorRed, "ERROR: "+err.Error()))
				return
			}
		} else {
			tooManyOpenFiles = true
			return
		}
	} else {
		signal.TotalRequest <- true
		if showData {
			b, err := ioutil.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				logsData.WriteString(zlog.ColorTextWrap(zlog.ColorYellow, "      Error: "))
				logsData.WriteString(err.Error())
			} else {
				// logsData.WriteString(util.Log.ColorTextWrap(util.Log.ColorCyan, "    Response: \n    "))
				logsData.WriteString("      ")
				logsData.WriteString(zlog.ColorTextWrap(zlog.ColorLightGrey, strings.Replace(zstring.Bytes2String(b), "\n", "\n      ", -1)))
			}
			logsData.WriteString("\n")
		} else {
			_, _ = io.Copy(ioutil.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		logsData.WriteString(" ")
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		if resp.StatusCode >= 200 && resp.StatusCode < 210 {
			logsData.WriteString(zlog.ColorTextWrap(zlog.ColorGreen, strconv.Itoa(resp.StatusCode)))
		} else {
			logsData.WriteString(zlog.ColorTextWrap(zlog.ColorRed, strconv.Itoa(resp.StatusCode)))
		}
	}

	// totalTime := t7.Sub(t0)
	totalTime := t7.Sub(t0) - t2.Sub(t0)
	if httpError {
		totalTime = -1
		// totalTime = timeout
	}
	diffTime = totalTime.Seconds()
	logsData.WriteString(" | ")
	if httpError {
		logsData.WriteString(fmt.Sprintf("%.2f", diffTime))
	} else {

		logsData.WriteString(fmt.Sprintf("%.3f", diffTime))
	}
	logsData.WriteString("s")
	logsData.WriteString(" ==> ")
	logsData.WriteString(req.Method)
	logsData.WriteString(" ")
	logsData.WriteString(req.Host)
	if req.URL.RawQuery != "" {
		logsData.WriteString(req.URL.Path)
		logsData.WriteString("?")
		logsData.WriteString(req.URL.RawQuery)
	}

	signal.MaxMinValue <- diffTime
	fmta := func(d time.Duration) string {
		return fmt.Sprintf("%7dms", int(d/time.Millisecond))
	}

	fmtb := func(d time.Duration) string {
		return fmt.Sprintf("%-9s", strconv.Itoa(int(d/time.Millisecond))+"ms")
	}
	fmtRes := ""
	u, _ := url.Parse(reqData.RawUrl)
	switch u.Scheme {
	case "https":
		fmtRes = fmt.Sprintf(httpsTemplate,
			fmta(t1.Sub(t0)), // dns lookup
			fmta(t2.Sub(t1)), // tcp connection
			fmta(t6.Sub(t5)), // tls handshake
			fmta(t4.Sub(t3)), // server processing
			fmta(t7.Sub(t4)), // content transfer
			fmtb(t1.Sub(t0)), // namelookup
			fmtb(t2.Sub(t0)), // connect
			fmtb(t3.Sub(t0)), // pretransfer
			fmtb(t4.Sub(t0)), // starttransfer
			fmtb(totalTime),  // total
		)
	case "http":
		fmtRes = fmt.Sprintf(httpTemplate,
			fmta(t1.Sub(t0)), // dns lookup
			fmta(t3.Sub(t1)), // tcp connection
			fmta(t4.Sub(t3)), // server processing
			fmta(t7.Sub(t4)), // content transfer
			fmtb(t1.Sub(t0)), // namelookup
			fmtb(t3.Sub(t0)), // connect
			fmtb(t4.Sub(t0)), // starttransfer
			fmtb(totalTime),  // total
		)
	}
	if !hideLog {
		if log := strings.Trim(logsData.String(), " "); log != "" {
			if showStat && !httpError {
				log = fmt.Sprintf("%s\n"+zlog.ColorTextWrap(zlog.ColorLightYellow, "%s%s"), log, connectedInfo.String(), fmtRes)
			}
			util.Log.Printf("%s\n============================================\n", log)
		}
	}

	return
}

func isDuration(startTime time.Time, fn func()) {
	diffTime := time.Now().Sub(startTime)
	appendTime := duration - diffTime
	if appendTime > 0 {
		// time.Sleep(10 * time.Millisecond)
		// diffTimeSeconds := diffTime.Seconds()
		// transactionRate := float64(TotalRequest) / diffTimeSeconds
		// concurrency := int(transactionRate * math.Floor(appendTime.Seconds()))
		i := uint(0)
		for i < concurrency {
			fn()
			i++
		}
		waitReq.Wait()
		isDuration(startTime, fn)
		// tick := time.NewTicker(appendTime)
		// for {
		// 	select {
		// 	case <-tick.C:
		// 		return
		// 	}
		// }
	}
}

func replaceMaxMinValue(ch <-chan float64) {
	defer waitStats.Done()

	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return
			}
			if v > LongestTransaction {
				LongestTransaction = v
			}
			if ShortestTransaction == 0 {
				ShortestTransaction = v
			} else if v < ShortestTransaction {
				ShortestTransaction = v
			}
		}
	}
}

func failedTransactionCount(ch <-chan bool) {
	defer waitStats.Done()

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			FailedTransactions += 1
		}
	}
}
func errorTransactionCount(ch <-chan bool) {
	defer waitStats.Done()

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			ErrorTransactions += 1
		}
	}
}

func totalRequestCount(ch <-chan bool) {
	defer waitStats.Done()

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			TotalRequest += 1
		}
	}
}
