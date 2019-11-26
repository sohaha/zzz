package stress

import (
	"fmt"
	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zzz/util"
	"strings"
	"time"
)

func Run(vars *Cli) {
	if *vars.RequestUrl == "" {
		example("RequestUrl cannot be empty")
	}
	
	if *vars.Concurrency < 1 {
		example("The number of concurrency cannot be less than 1")
	}
	timeout = time.Duration(*vars.Timeout) * time.Second
	duration = time.Duration(*vars.Duration) * time.Second
	showData = *vars.Debug
	showStat = *vars.Stat
	hideLog = *vars.Hidelog
	concurrency = uint(*vars.Concurrency)
	client(vars)
}

func client(vars *Cli) {
	var reqData RequestContext
	var signal Signal
	reqData.RawUrl = strings.TrimSpace(*vars.RequestUrl)
	
	if len(reqData.RawUrl) < 8 {
		reqData.RawUrl = "http://" + reqData.RawUrl
	} else if reqData.RawUrl[:7] != "http://" && reqData.RawUrl[:8] != "https://" {
		reqData.RawUrl = "http://" + reqData.RawUrl
	}
	
	if *vars.Header != "" {
		headerSlice := strings.Split(*vars.Header, "&")
		for _, v := range headerSlice {
			kv := strings.SplitN(v, ":", 2)
			if len(kv) != 2 {
				util.Log.Error("Header format error")
				return
			}
			var kvArray [2]string
			kvArray[0] = kv[0]
			kvArray[1] = kv[1]
			reqData.HeaderKVSlice = append(reqData.HeaderKVSlice, kvArray)
		}
		
	} else {
		reqData.HeaderKVSlice = nil
	}
	
	reqData.Body = strings.TrimSpace(*vars.Body)
	reqData.Method = strings.ToUpper(strings.TrimSpace(*vars.Method))
	
	signal.MaxMinValue = make(chan float64)
	signal.FailedTransactions = make(chan bool)
	signal.ErrorTransactions = make(chan bool)
	signal.TotalRequest = make(chan bool)
	
	waitStats.Add(4)
	go replaceMaxMinValue(signal.MaxMinValue)
	go failedTransactionCount(signal.FailedTransactions)
	go errorTransactionCount(signal.ErrorTransactions)
	go totalRequestCount(signal.TotalRequest)
	
	startTime := time.Now()
	
	waitReq.Add(int(concurrency))
	runFn := func() {
		var i uint = 0
		for i < concurrency {
			go httpSendRequest(&reqData, signal)
			i++
		}
	}
	
	runFn()
	if duration > 1 {
		continued := int(duration/time.Second) - 1
		waitReq.Add(int(concurrency) * continued)
		for i := continued; i > 0; i-- {
			time.AfterFunc(time.Duration(i)*time.Second, runFn)
		}
	}
	// todo 这个其实应该改成每秒执行一次
	// isDuration(startTime, func() {
	// 	waitReq.Add(1)
	// 	go httpSendRequest(&reqData, signal)
	// })
	
	waitReq.Wait()
	
	close(signal.MaxMinValue)
	close(signal.FailedTransactions)
	close(signal.ErrorTransactions)
	close(signal.TotalRequest)
	
	waitStats.Wait()
	
	ElapsedTime = time.Now().Sub(startTime).Seconds()
	SuccessTransactions := TotalRequest - FailedTransactions
	
	if SuccessTransactions == 0 || TotalRequest == 0 {
		SuccessRate = 0
	} else {
		SuccessRate = float64(SuccessTransactions) / float64(TotalRequest) * 100
	}
	
	TransactionRate = float64(TotalRequest) / ElapsedTime
	if tooManyOpenFiles {
		util.Log.Warn("Too many open files error:", "https://stackoverflow.com/questions/880557/socket-accept-too-many-open-files")
	}
	if !hideLog {
		util.Log.Println()
	}
	util.Log.Println("Total requests:    ", TotalRequest)
	util.Log.Println("Successful:        ", SuccessTransactions)
	
	if FailedTransactions > 0 {
		util.Log.Println(util.Log.ColorTextWrap(zlog.ColorRed, "Failures:           "+fmt.Sprintf("%d", FailedTransactions)))
	} else {
		util.Log.Println("Failures:          ", FailedTransactions)
	}
	if ErrorTransactions > 0 {
		util.Log.Println(util.Log.ColorTextWrap(zlog.ColorRed, "Error:              "+fmt.Sprintf("%d", ErrorTransactions)))
	}
	
	if SuccessRate < 100 {
		util.Log.Println(util.Log.ColorTextWrap(zlog.ColorRed, "Success rate:       "+fmt.Sprintf("%.2f", SuccessRate)+"%"))
	} else {
		util.Log.Println(util.Log.ColorTextWrap(zlog.ColorGreen, "Success rate:       "+fmt.Sprintf("%.2f", SuccessRate)+"%"))
	}
	util.Log.Println("Average processed: ", fmt.Sprintf("%.2f", TransactionRate)+" Times/second")
	util.Log.Println("Longest time:      ", fmt.Sprintf("%.3f", LongestTransaction)+" second")
	util.Log.Println("Shortest time:     ", fmt.Sprintf("%.3f", ShortestTransaction)+" second")
	util.Log.Println("Total time:        ", fmt.Sprintf("%.3f", ElapsedTime)+" second")
	util.Log.Println()
}

func example(errMsg ...string) {
	util.Log.Error(errMsg[0])
}
