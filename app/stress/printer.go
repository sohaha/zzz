package stress

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"sync"

	"github.com/sohaha/zlsgo/zfile"

	color "github.com/fatih/color"
)

type printer struct {
	output    io.Writer
	writeLock sync.Mutex
}

// CreateTextStressSummary creates a human friendly summary of entire stress test
func CreateTextStressSummary(reqStatSummary RequestStatSummary) string {
	summary := "\n"

	summary += "时间统计\n"
	summary += fmt.Sprintf("平均查询速度:     %d ms\n", reqStatSummary.avgDuration/1000000)
	summary += fmt.Sprintf("最快查询速度:     %d ms\n", reqStatSummary.minDuration/1000000)
	summary += fmt.Sprintf("最慢查询速度:     %d ms\n", reqStatSummary.maxDuration/1000000)
	summary += fmt.Sprintf("平均 RPS:         %.2f req/sec\n", reqStatSummary.avgRPS*1000000000)
	summary += fmt.Sprintf("总耗时:           %d ms\n", reqStatSummary.endTime.Sub(reqStatSummary.startTime).Nanoseconds()/1000000)

	summary += "\n数据传输\n"
	summary += fmt.Sprintf("平均查询:      %s\n", zfile.SizeFormat(int64(reqStatSummary.avgDataTransferred)))
	summary += fmt.Sprintf("最大查询:      %s\n", zfile.SizeFormat(int64(reqStatSummary.maxDataTransferred)))
	summary += fmt.Sprintf("最小查询:      %s\n", zfile.SizeFormat(int64(reqStatSummary.minDataTransferred)))
	summary += fmt.Sprintf("总计:          %s\n", zfile.SizeFormat(int64(reqStatSummary.totalDataTransferred)))

	summary = summary + "\n响应代码\n"
	// sort the status codes
	var codes []int
	totalResponses := 0
	for key, val := range reqStatSummary.statusCodes {
		codes = append(codes, key)
		totalResponses += val
	}
	sort.Ints(codes)
	for _, code := range codes {
		if code == 0 {
			summary += "失败"
		} else {
			summary += fmt.Sprintf("%d", code)
		}
		summary += ": " + fmt.Sprintf("%d", reqStatSummary.statusCodes[code])
		if code == 0 {
			summary += " 请求"
		} else {
			summary += " 响应"
		}
		summary += " (" + fmt.Sprintf("%.2f", 100*float64(reqStatSummary.statusCodes[code])/float64(totalResponses)) + "%)\n"
	}
	return summary
}

func (p *printer) printStat(stat RequestStat) {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()

	if stat.Error != nil {
		color.Set(color.FgRed)
		fmt.Fprintln(p.output, "请求失败: "+stat.Error.Error())
		color.Unset()
		return
	}

	if stat.StatusCode >= 100 && stat.StatusCode < 200 {
		color.Set(color.FgBlue)
	} else if stat.StatusCode >= 200 && stat.StatusCode < 300 {
		color.Set(color.FgGreen)
	} else if stat.StatusCode >= 300 && stat.StatusCode < 400 {
		color.Set(color.FgCyan)
	} else if stat.StatusCode >= 400 && stat.StatusCode < 500 {
		color.Set(color.FgMagenta)
	} else {
		color.Set(color.FgRed)
	}
	fmt.Fprintf(p.output, "%s %d\t%s \t%d ms\t-> %s %s\n",
		stat.Proto,
		stat.StatusCode,
		zfile.SizeFormat(int64(stat.DataTransferred)),
		stat.Duration.Nanoseconds()/1000000,
		stat.Method,
		stat.URL)
	color.Unset()
}

// print tons of info about the request, response and response body
func (p *printer) printVerbose(req *http.Request, response *http.Response) {
	if req == nil {
		return
	}
	if response == nil {
		return
	}
	var requestInfo string
	// request details
	requestInfo = requestInfo + fmt.Sprintf("请求:\n%+v\n\n", &req)

	// reponse metadata
	requestInfo = requestInfo + fmt.Sprintf("响应:\n%+v\n\n", response)

	// reponse body
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		requestInfo = requestInfo + fmt.Sprintf("读取响应体失败: %s\n", err.Error())
	} else {
		requestInfo = requestInfo + fmt.Sprintf("响应体:\n%s\n\n", body)
		_ = response.Body.Close()
	}
	p.writeLock.Lock()
	_, _ = fmt.Fprintln(p.output, requestInfo)
	p.writeLock.Unlock()
}

// writeString is a generic output string printer
func (p *printer) writeString(s string) {
	p.writeLock.Lock()
	fmt.Fprint(p.output, s)
	p.writeLock.Unlock()
}
