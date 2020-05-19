package cmd

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/dustin/go-humanize"
	"github.com/sohaha/zzz/app/stress"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var stressUse = "stress"
var stressCfg string

// copyright: https://github.com/bengadbois/pewpew
var stressCmd = &cobra.Command{
	Use:     stressUse,
	Short:   "Run stress tests",
	Aliases: []string{"s"},
	// 	Example: fmt.Sprintf(`  %s %s www.baidu.com
	//   %[1]s %[2]s -c 10 -t 10 -u https://www.baidu.com`, use, stressUse),
	RunE: func(cmd *cobra.Command, args []string) error {
		viper.SetConfigName("zzz-stress")
		viper.AddConfigPath(".")
		err := viper.ReadInConfig()
		if err != nil {
			if _, ok := err.(viper.ConfigParseError); ok {
				fmt.Println("Failed to parse config file " + viper.ConfigFileUsed())
				fmt.Println(err)
				os.Exit(-1)
			}
		}
		if viper.ConfigFileUsed() != "" {
			fmt.Println("Using config file: " + viper.ConfigFileUsed())
		}
		err = viper.BindPFlag("count", cmd.Flags().Lookup("num"))
		if err != nil {
			fmt.Println("failed to configure flags")
			fmt.Println(err)
			os.Exit(-1)
		}

		err = viper.BindPFlag("concurrency", cmd.Flags().Lookup("concurrent"))
		if err != nil {
			fmt.Println("failed to configure flags")
			fmt.Println(err)
			os.Exit(-1)
		}
		err = viper.BindPFlags(cmd.PersistentFlags())
		if err != nil {
			fmt.Println("failed to configure flags")
			fmt.Println(err)
			os.Exit(-1)
		}

		stressCfg := stress.StressConfig{}
		err = viper.Unmarshal(&stressCfg)
		if err != nil {
			return errors.New("could not parse config file")
		}
		stressCfg.Quiet = viper.GetBool("quiet")
		stressCfg.Verbose = viper.GetBool("verbose")
		stressCfg.Count = viper.GetInt("count")
		stressCfg.Concurrency = viper.GetInt("concurrency")

		if len(stressCfg.Targets) == 0 && len(args) < 1 {
			return cmd.Help()
		}

		// if URLs are set on command line, use that for Targets instead of config
		if len(args) >= 1 {
			stressCfg.Targets = make([]stress.Target, len(args))
			for i := range stressCfg.Targets {
				stressCfg.Targets[i].URL = args[i]
				stressCfg.Targets[i].RegexURL, _ = cmd.Flags().GetBool("regex")
				stressCfg.Targets[i].DNSPrefetch, _ = cmd.Flags().GetBool("dns-prefetch")
				stressCfg.Targets[i].Timeout, _ = cmd.Flags().GetString("timeout")
				stressCfg.Targets[i].Method, _ = cmd.Flags().GetString("request-method")
				stressCfg.Targets[i].Body, _ = cmd.Flags().GetString("body")
				stressCfg.Targets[i].BodyFilename, _ = cmd.Flags().GetString("body-file")
				stressCfg.Targets[i].Headers, _ = cmd.Flags().GetString("headers")
				stressCfg.Targets[i].Cookies, _ = cmd.Flags().GetString("cookies")
				stressCfg.Targets[i].UserAgent, _ = cmd.Flags().GetString("user-agent")
				stressCfg.Targets[i].BasicAuth, _ = cmd.Flags().GetString("basic-auth")
				stressCfg.Targets[i].Compress, _ = cmd.Flags().GetBool("compress")
				stressCfg.Targets[i].KeepAlive, _ = cmd.Flags().GetBool("keepalive")
				stressCfg.Targets[i].FollowRedirects, _ = cmd.Flags().GetBool("follow-redirects")
				stressCfg.Targets[i].NoHTTP2, _ = cmd.Flags().GetBool("no-http2")
				stressCfg.Targets[i].EnforceSSL, _ = cmd.Flags().GetBool("enforce-ssl")
			}
		} else {
			for i, target := range viper.Get("targets").([]interface{}) {
				targetMapVals := make(map[string]interface{})
				for key, value := range target.(map[interface{}]interface{}) {
					strKey := fmt.Sprintf("%v", key)
					targetMapVals[strKey] = value
				}
				if _, set := targetMapVals["RegexURL"]; !set {
					stressCfg.Targets[i].RegexURL, _ = cmd.Flags().GetBool("regex")
				}
				if _, set := targetMapVals["DNSPrefetch"]; !set {
					stressCfg.Targets[i].DNSPrefetch, _ = cmd.Flags().GetBool("dns-prefetch")
				}
				if _, set := targetMapVals["Timeout"]; !set {
					stressCfg.Targets[i].Timeout, _ = cmd.Flags().GetString("timeout")
				}
				if _, set := targetMapVals["Method"]; !set {
					stressCfg.Targets[i].Method, _ = cmd.Flags().GetString("request-method")
				}
				if _, set := targetMapVals["Body"]; !set {
					stressCfg.Targets[i].Body, _ = cmd.Flags().GetString("body")
				}
				if _, set := targetMapVals["BodyFilename"]; !set {
					stressCfg.Targets[i].BodyFilename, _ = cmd.Flags().GetString("bodyFile")
				}
				if _, set := targetMapVals["Headers"]; !set {
					stressCfg.Targets[i].Headers, _ = cmd.Flags().GetString("headers")
				}
				if _, set := targetMapVals["Cookies"]; !set {
					stressCfg.Targets[i].Cookies, _ = cmd.Flags().GetString("cookies")
				}
				if _, set := targetMapVals["UserAgent"]; !set {
					stressCfg.Targets[i].UserAgent, _ = cmd.Flags().GetString("userAgent")
				}
				if _, set := targetMapVals["BasicAuth"]; !set {
					stressCfg.Targets[i].BasicAuth, _ = cmd.Flags().GetString("basicAuth")
				}
				if _, set := targetMapVals["Compress"]; !set {
					stressCfg.Targets[i].Compress, _ = cmd.Flags().GetBool("compress")
				}
				if _, set := targetMapVals["KeepAlive"]; !set {
					stressCfg.Targets[i].KeepAlive, _ = cmd.Flags().GetBool("keepalive")
				}
				if _, set := targetMapVals["FollowRedirects"]; !set {
					stressCfg.Targets[i].FollowRedirects, _ = cmd.Flags().GetBool("followredirects")
				}
				if _, set := targetMapVals["NoHTTP2"]; !set {
					stressCfg.Targets[i].NoHTTP2, _ = cmd.Flags().GetBool("no-http2")
				}
				if _, set := targetMapVals["EnforceSSL"]; !set {
					stressCfg.Targets[i].EnforceSSL, _ = cmd.Flags().GetBool("enforce-ssl")
				}
			}
		}

		targetRequestStats, err := stress.RunStress(stressCfg, os.Stdout)
		if err != nil {
			return err
		}

		fmt.Print("\n----Summary----\n\n")

		// only print individual target data if multiple targets
		if len(stressCfg.Targets) > 1 {
			for idx, target := range stressCfg.Targets {
				// info about the request
				fmt.Printf("----Target %d: %s %s\n", idx+1, target.Method, target.URL)
				reqStats := stress.CreateRequestsStats(targetRequestStats[idx])
				fmt.Println(stress.CreateTextStressSummary(reqStats))
			}
		}

		// combine individual targets to a total one
		globalStats := []stress.RequestStat{}
		for i := range stressCfg.Targets {
			for j := range targetRequestStats[i] {
				globalStats = append(globalStats, targetRequestStats[i][j])
			}
		}
		if len(stressCfg.Targets) > 1 {
			fmt.Println("----Global----")
		}
		reqStats := stress.CreateRequestsStats(globalStats)
		fmt.Println(stress.CreateTextStressSummary(reqStats))

		if viper.GetString("output-json") != "" {
			filename := viper.GetString("output-json")
			fmt.Print("Writing full result data to: " + filename + " ...")
			json, _ := json.MarshalIndent(globalStats, "", "    ")
			err = ioutil.WriteFile(filename, json, 0644)
			if err != nil {
				return errors.New("failed to write full result data to " +
					filename + ": " + err.Error())
			}
			fmt.Println("finished!")
		}
		// write out csv
		if viper.GetString("output-csv") != "" {
			filename := viper.GetString("output-csv")
			fmt.Print("Writing full result data to: " + filename + " ...")
			file, err := os.Create(filename)
			if err != nil {
				return errors.New("failed to write full result data to " +
					filename + ": " + err.Error())
			}
			defer file.Close()

			writer := csv.NewWriter(file)

			for _, req := range globalStats {
				line := []string{
					req.StartTime.String(),
					fmt.Sprintf("%d", req.Duration),
					fmt.Sprintf("%d", req.StatusCode),
					humanize.Bytes(uint64(req.DataTransferred)),
				}
				err := writer.Write(line)
				if err != nil {
					return errors.New("failed to write full result data to " +
						filename + ": " + err.Error())
				}
			}
			defer writer.Flush()
			fmt.Println("finished!")
		}

		if viper.GetString("output-xml") != "" {
			filename := viper.GetString("output-xml")
			fmt.Print("Writing full result data to: " + filename + " ...")
			xml, _ := xml.MarshalIndent(globalStats, "", "    ")
			err = ioutil.WriteFile(viper.GetString("output-xml"), xml, 0644)
			if err != nil {
				return errors.New("failed to write full result data to " +
					filename + ": " + err.Error())
			}
			fmt.Println("finished!")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stressCmd)
	stressCmd.Flags().BoolP("regex", "r", false, "Interpret URLs as regular expressions.")
	stressCmd.Flags().Bool("dns-prefetch", false, "Prefetch IP from hostname before making request, eliminating DNS fetching from timing.")
	stressCmd.Flags().StringP("timeout", "t", "10s", "Maximum seconds to wait for response")
	stressCmd.Flags().StringP("request-method", "X", "GET", "Request type. GET, HEAD, POST, PUT, etc.")
	stressCmd.Flags().String("body", "", "String to use as request body e.g. POST body.")
	stressCmd.Flags().String("body-file", "", "Path to file to use as request body. Will overwrite --body if both are present.")
	stressCmd.Flags().StringP("headers", "H", "", "Add arbitrary header line, eg. 'Accept-Encoding:gzip, Content-Type:application/json'")
	stressCmd.Flags().String("cookies", "", "Add request cookies, eg. 'data=123; session=456'")
	stressCmd.Flags().StringP("user-agent", "A", "zzz-stress", "Add User-Agent header. Can also be done with the arbitrary header flag.")
	stressCmd.Flags().String("basic-auth", "", "Add HTTP basic authentication, eg. 'user123:password456'.")
	stressCmd.Flags().BoolP("compress", "C", true, "Add 'Accept-Encoding: gzip' header if Accept-Encoding is not already present.")
	stressCmd.Flags().BoolP("keepalive", "k", true, "Enable HTTP KeepAlive.")
	stressCmd.Flags().Bool("follow-redirects", true, "Follow HTTP redirects.")
	stressCmd.Flags().Bool("no-http2", false, "Disable HTTP2.")
	stressCmd.Flags().Bool("enforce-ssl", false, "Enfore SSL certificate correctness.")
	stressCmd.Flags().String("output-json", "", "Path to file to write full data as JSON")
	stressCmd.Flags().String("output-csv", "", "Path to file to write full data as CSV")
	stressCmd.Flags().String("output-xml", "", "Path to file to write full data as XML")
	stressCmd.Flags().BoolP("quiet", "q", false, "Do not print while requests are running.")
	stressCmd.Flags().Int("cpu", runtime.GOMAXPROCS(0), "Number of CPUs to use.")
	stressCmd.Flags().IntP("concurrent", "c", stress.DefaultConcurrency, "Number of concurrent requests to make.")
	stressCmd.Flags().IntP("num", "n", stress.DefaultCount, "Number of total requests to make.")
	stress.InitCmd(stressCmd)
	stressCmd.PersistentFlags().StringVar(&stressCfg, "cfg", "./zzz-stress.yml", "Stress config file path")
}
