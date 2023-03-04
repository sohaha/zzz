package stress

import (
	"errors"
	"time"
)

// Reasonable default values for a target
const (
	DefaultURL         = "http://localhost"
	DefaultTimeout     = "10s"
	DefaultMethod      = "GET"
	DefaultUserAgent   = "stress"
	DefaultCount       = 10
	DefaultConcurrency = 1
)

type (
	// Target is location of where send the HTTP request and how to send it.
	Target struct {
		Cookies         string
		BasicAuth       string
		UserAgent       string
		Timeout         string
		Method          string
		Body            string
		BodyFilename    string
		Headers         string
		URL             string
		DNSPrefetch     bool
		RegexURL        bool
		Compress        bool
		KeepAlive       bool
		FollowRedirects bool
		NoHTTP2         bool
		EnforceSSL      bool
	}
)

func validateTarget(target Target) error {
	if target.URL == "" {
		return errors.New("empty URL")
	}
	if target.Method == "" {
		return errors.New("method cannot be empty string")
	}
	if target.Timeout != "" {
		timeout, err := time.ParseDuration(target.Timeout)
		if err != nil {
			return errors.New("failed to parse timeout: " + target.Timeout)
		}
		if timeout <= time.Millisecond {
			return errors.New("timeout must be greater than one millisecond")
		}
	}
	return nil
}
