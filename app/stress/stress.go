package stress

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// TODO move to other file
type workerDone struct{}

type (
	// StressConfig is the top level struct that contains the configuration for a stress test
	StressConfig struct {
		Cookies         string
		BasicAuth       string
		Body            string
		BodyFilename    string
		Headers         string
		UserAgent       string
		Timeout         string
		Method          string
		Targets         []Target
		Count           int
		Concurrency     int
		Verbose         bool
		DNSPrefetch     bool
		Quiet           bool
		Compress        bool
		KeepAlive       bool
		FollowRedirects bool
		NoHTTP2         bool
		EnforceSSL      bool
	}
)

// NewStressConfig creates a new StressConfig
// with package defaults
func NewStressConfig() (s *StressConfig) {
	s = &StressConfig{
		Count:       DefaultCount,
		Concurrency: DefaultConcurrency,
		Targets: []Target{
			{
				URL:             DefaultURL,
				Timeout:         DefaultTimeout,
				Method:          DefaultMethod,
				UserAgent:       DefaultUserAgent,
				FollowRedirects: true,
			},
		},
	}
	return
}

// RunStress starts the stress tests with the provided StressConfig.
// Throughout the test, data is sent to w, useful for live updates.
func RunStress(s StressConfig, w io.Writer) ([][]RequestStat, error) {
	if w == nil {
		return nil, errors.New("写入器为空")
	}
	err := validateStressConfig(s)

	if err != nil {
		return nil, errors.New("配置无效: " + err.Error())
	}
	targetCount := len(s.Targets)

	// setup printer
	p := printer{output: w}

	// setup the queue of requests, one queue per target
	requestQueues := make([]chan http.Request, targetCount)
	for idx, target := range s.Targets {
		requestQueue, err := createRequestQueue(s.Count, target)
		if err != nil {
			return nil, err
		}
		requestQueues[idx] = requestQueue
	}

	if targetCount == 1 {
		_, _ = fmt.Fprintf(w, "压测 %d 个目标:\n", targetCount)
	} else {
		_, _ = fmt.Fprintf(w, "压测 %d 个目标:\n", targetCount)
	}

	// when a target is finished, send all stats into this
	targetStats := make(chan []RequestStat)
	for idx, target := range s.Targets {
		go func(target Target, requestQueue chan http.Request, targetStats chan []RequestStat) {
			p.writeString(fmt.Sprintf("- Running %d tests at %s, %d at a time\n", s.Count, target.URL, s.Concurrency))

			workerDoneChan := make(chan workerDone)   // workers use this to indicate they are done
			requestStatChan := make(chan RequestStat) // workers communicate each requests' info

			// start up the workers
			for i := 0; i < s.Concurrency; i++ {
				go func() {
					client := createClient(target)
					defer client.CloseIdleConnections()
					// todo We need to optimize. There are too many requests at one time
					for req := range requestQueue {
						response, stat := runRequest(req, client)
						if !s.Quiet {
							p.printStat(stat)
							if s.Verbose {
								p.printVerbose(&req, response)
							}
						}
						if stat.Error == nil {
							if !s.Verbose {
								_, _ = io.Copy(ioutil.Discard, response.Body)
							}
							err = response.Body.Close()
						}
						requestStatChan <- stat
					}
					workerDoneChan <- workerDone{}
				}()
			}
			requestStats := make([]RequestStat, s.Count)
			requestsCompleteCount := 0
			workersDoneCount := 0
			// wait for all workers to finish
			for {
				select {
				case <-workerDoneChan:
					workersDoneCount++
				case stat := <-requestStatChan:
					requestStats[requestsCompleteCount] = stat
					requestsCompleteCount++
				}
				if workersDoneCount == s.Concurrency {
					// all workers are finished
					break
				}
			}
			targetStats <- requestStats
		}(target, requestQueues[idx], targetStats)
	}
	targetRequestStats := make([][]RequestStat, targetCount)
	targetDoneCount := 0
	for reqStats := range targetStats {
		targetRequestStats[targetDoneCount] = reqStats
		targetDoneCount++
		if targetDoneCount == targetCount {
			// all targets are finished
			break
		}
	}

	return targetRequestStats, nil
}

func validateStressConfig(s StressConfig) error {
	if len(s.Targets) == 0 {
		return errors.New("目标数量为零")
	}
	if s.Count <= 0 {
		return errors.New("请求数量必须大于零")
	}
	if s.Concurrency <= 0 {
		return errors.New("并发数必须大于零")
	}
	if s.Concurrency > s.Count {
		return errors.New("并发数不能超过请求总数")
	}

	for _, target := range s.Targets {
		if err := validateTarget(target); err != nil {
			return err
		}
	}
	return nil
}

// createRequestQueue creates a channel of http.Requests of size count
func createRequestQueue(count int, target Target) (chan http.Request, error) {
	requestQueue := make(chan http.Request)
	// attempt to build one request - if passes, the rest should too
	_, err := buildRequest(target)
	if err != nil {
		return nil, errors.New("使用目标配置创建请求失败: " + err.Error())
	}
	go func() {
		for i := 0; i < count; i++ {
			req, err := buildRequest(target)
			if err != nil {
				// this shouldn't happen, but probably should handle for it
				continue
			}
			requestQueue <- req
		}
		close(requestQueue)
	}()
	return requestQueue, nil
}
