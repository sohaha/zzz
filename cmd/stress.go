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
	"github.com/spf13/cobra"
	v "github.com/spf13/viper"

	"github.com/sohaha/zlsgo/ztype"
	"github.com/sohaha/zzz/app/stress"
	"github.com/sohaha/zzz/util"
)

var (
	stressUse = "stress"
	stressCfg string
)

// copyright: https://github.com/bengadbois/pewpew
var stressCmd = &cobra.Command{
	Use:     stressUse,
	Short:   "运行压测",
	Aliases: []string{"s"},
	// 	Example: fmt.Sprintf(`  %s %s www.baidu.com
	//   %[1]s %[2]s -c 10 -t 10 -u https://www.baidu.com`, use, stressUse),
	RunE: func(cmd *cobra.Command, args []string) error {
		viper := v.New()
		viper.SetConfigName("zzz-stress")
		viper.AddConfigPath(".")
		err := viper.ReadInConfig()
		if err != nil {
			if _, ok := err.(v.ConfigParseError); ok {
				fmt.Println("解析配置文件失败: " + viper.ConfigFileUsed())
				fmt.Println(err)
				os.Exit(-1)
			}
		}
		if viper.ConfigFileUsed() != "" {
			fmt.Println("使用配置文件: " + viper.ConfigFileUsed())
		}
		err = viper.BindPFlag("count", cmd.Flags().Lookup("num"))
		if err != nil {
			fmt.Println("绑定参数失败")
			fmt.Println(err)
			os.Exit(-1)
		}

		err = viper.BindPFlag("concurrency", cmd.Flags().Lookup("concurrent"))
		if err != nil {
			fmt.Println("绑定参数失败")
			fmt.Println(err)
			os.Exit(-1)
		}
		err = viper.BindPFlags(cmd.PersistentFlags())
		if err != nil {
			fmt.Println("绑定参数失败")
			fmt.Println(err)
			os.Exit(-1)
		}

		stressCfg := stress.StressConfig{}
		err = viper.Unmarshal(&stressCfg)
		if err != nil {
			return errors.New("无法解析配置文件")
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
				for key, value := range ztype.ToMap(target) {
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
					stressCfg.Targets[i].BodyFilename, _ = cmd.Flags().GetString("body-file")
				}
				if _, set := targetMapVals["Headers"]; !set {
					stressCfg.Targets[i].Headers, _ = cmd.Flags().GetString("headers")
				}
				if _, set := targetMapVals["Cookies"]; !set {
					stressCfg.Targets[i].Cookies, _ = cmd.Flags().GetString("cookies")
				}
				if _, set := targetMapVals["UserAgent"]; !set {
					stressCfg.Targets[i].UserAgent, _ = cmd.Flags().GetString("user-agent")
				}
				if _, set := targetMapVals["BasicAuth"]; !set {
					stressCfg.Targets[i].BasicAuth, _ = cmd.Flags().GetString("basic-auth")
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

		util.SetLimit(999999)
		targetRequestStats, err := stress.RunStress(stressCfg, os.Stdout)
		if err != nil {
			return err
		}

		fmt.Print("\n----汇总----\n\n")

		// only print individual target data if multiple targets
		if len(stressCfg.Targets) > 1 {
			for idx, target := range stressCfg.Targets {
				// info about the request
				fmt.Printf("----目标 %d: %s %s\n", idx+1, target.Method, target.URL)
				reqStats := stress.CreateRequestsStats(targetRequestStats[idx])
				fmt.Println(stress.CreateTextStressSummary(reqStats))
			}
		}

		// combine individual targets to a total one
		globalStats := make([]stress.RequestStat, 0)
		for i := range stressCfg.Targets {
			for j := range targetRequestStats[i] {
				globalStats = append(globalStats, targetRequestStats[i][j])
			}
		}
		if len(stressCfg.Targets) > 1 {
			fmt.Println("----全局统计----")
		}
		reqStats := stress.CreateRequestsStats(globalStats)
		fmt.Println(stress.CreateTextStressSummary(reqStats))

		if viper.GetString("output-json") != "" {
			filename := viper.GetString("output-json")
			fmt.Print("正在将完整结果数据写入: " + filename + " ...")
			json, _ := json.MarshalIndent(globalStats, "", "    ")
			err = ioutil.WriteFile(filename, json, 0644)
			if err != nil {
				return errors.New("写入完整结果数据失败: " +
					filename + ": " + err.Error())
			}
			fmt.Println("写入完成!")
		}
		// write out csv
		if viper.GetString("output-csv") != "" {
			filename := viper.GetString("output-csv")
			fmt.Print("正在写入 CSV 结果到: " + filename + " ...")
			file, err := os.Create(filename)
			if err != nil {
				return errors.New("写入完整结果数据失败: " +
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
					return errors.New("写入完整结果数据失败: " +
						filename + ": " + err.Error())
				}
			}
			defer writer.Flush()
			fmt.Println("写入完成!")
		}

		if viper.GetString("output-xml") != "" {
			filename := viper.GetString("output-xml")
			fmt.Print("正在写入 XML 结果到: " + filename + " ...")
			xml, _ := xml.MarshalIndent(globalStats, "", "    ")
			err = ioutil.WriteFile(viper.GetString("output-xml"), xml, 0644)
			if err != nil {
				return errors.New("写入完整结果数据失败: " +
					filename + ": " + err.Error())
			}
			fmt.Println("写入完成!")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stressCmd)
	stressCmd.Flags().BoolP("regex", "r", false, "将目标 URL 视为正则表达式")
	stressCmd.Flags().Bool("dns-prefetch", false, "请求前预解析 DNS，避免计时包含 DNS 解析")
	stressCmd.Flags().StringP("timeout", "t", "10s", "等待响应的最长时间")
	stressCmd.Flags().StringP("request-method", "X", "GET", "请求方法，如 GET、HEAD、POST、PUT 等")
	stressCmd.Flags().String("body", "", "作为请求 Body 的字符串，例如 POST 数据")
	stressCmd.Flags().String("body-file", "", "从文件读取请求 Body（同时提供时优先于 --body）")
	stressCmd.Flags().StringP("headers", "H", "", "附加自定义请求头，如 'Accept-Encoding:gzip, Content-Type:application/json'")
	stressCmd.Flags().String("cookies", "", "附加请求 Cookie，如 'data=123; session=456'")
	stressCmd.Flags().StringP("user-agent", "A", "zzz-stress", "自定义 User-Agent 头")
	stressCmd.Flags().String("basic-auth", "", "HTTP 基础认证，如 'user123:password456'")
	stressCmd.Flags().BoolP("compress", "C", true, "若未指定则添加 'Accept-Encoding: gzip' 请求头")
	stressCmd.Flags().BoolP("keepalive", "k", true, "启用 HTTP KeepAlive")
	stressCmd.Flags().Bool("follow-redirects", true, "跟随 HTTP 跳转")
	stressCmd.Flags().Bool("no-http2", false, "禁用 HTTP/2")
	stressCmd.Flags().Bool("enforce-ssl", false, "严格校验证书正确性")
	stressCmd.Flags().String("output-json", "", "将完整结果写入 JSON 文件")
	stressCmd.Flags().String("output-csv", "", "将完整结果写入 CSV 文件")
	stressCmd.Flags().String("output-xml", "", "将完整结果写入 XML 文件")
	stressCmd.Flags().BoolP("quiet", "q", false, "执行过程中不打印输出")
	stressCmd.Flags().Int("cpu", runtime.GOMAXPROCS(0), "使用的 CPU 数量")
	stressCmd.Flags().IntP("concurrent", "c", stress.DefaultConcurrency, "并发请求数")
	stressCmd.Flags().IntP("num", "n", stress.DefaultCount, "总请求数")
	stress.InitCmd(stressCmd)
	stressCmd.PersistentFlags().StringVar(&stressCfg, "cfg", "./zzz-stress.yml", "压测配置文件路径")
}
