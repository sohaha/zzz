package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zhttp"
	"github.com/sohaha/zlsgo/zjson"

	initApp "github.com/sohaha/zzz/app/init"
	"github.com/sohaha/zzz/util"
)

var (
	createUes   = "init"
	createName  string
	createForce bool
	createList  bool
	gitType     = "github"
	createCmd   = &cobra.Command{
		Use:   createUes,
		Short: "初始化新项目模板",
		Long:  ``,
		// Args:cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			argsL := len(args)
			tmp := ""
			if argsL >= 2 {
				tmp = args[1]
			} else {
				depots := map[string]string{
					"zlsgo-app": "https://api.github.com/repos/sohaha/zlsgo-app/branches",
					// "ZlsPHP": "https://api.github.com/repos/sohaha/ZlsPHP/branches",
				}
				var branches []string
				for k, v := range depots {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					res, err := zhttp.Get(v, ctx)
					cancel()
					handle := func(res *zhttp.Res) {
						body := res.Bytes()
						zjson.ParseBytes(body).ForEach(func(key, value *zjson.Res) bool {
							name := value.Get("name").String()
							if name == "dev" && strings.HasSuffix(name, "-old") {
								return true
							}
							if name == "" {
								return false
							}
							branches = append(branches, k+":"+name)
							return true
						})
					}
					if err == nil {
						handle(res)
					} else {
						util.Log.Warn("请求超时，正在重试")
						res, err = zhttp.Get("https://github.73zls.com/" + v)
						if err == nil {
							handle(res)
						}
					}
				}
				if len(branches) == 0 {
					util.Log.Error("获取模板列表失败，请检查网络连接")
					return
				}
				prompt := promptui.Select{
					Label: "选择模板",
					Items: branches,
				}
				_, result, err := prompt.Run()
				if err == nil {
					tmp = "sohaha/" + result
				}
			}
			temples := strings.Split(tmp, ":")
			branch := "master"
			name := tmp
			if len(temples) >= 2 {
				branch = temples[1]
				name = temples[0]
				// name = strings.Join(temples[:2], "/")
			}
			dir := ""
			if argsL > 0 {
				dir = zfile.RealPath(args[0])
			} else {
				dir = "."
				if len(temples) >= 2 {
					dir = temples[1]
					// for _, v := range []string{"main", "master"} {
					// 	if dir == v {
					// 		dir = name
					// 		break
					// 	}
					// }
				}
				dir = zfile.RealPath(dir)
			}
			if name == "" {
				return
			}
			util.Log.Info("开始下载模板...")
			err := initApp.Clone(dir, name, branch)
			if err != nil {
				util.Log.Fatal(err)
				return
			}
			util.Log.Successf("初始化完成：%s\n", dir)
		},
	}
)

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Example = fmt.Sprintf(`  %[1]s %[2]s
  %[1]s %[2]s [dir] [name]`, use, createUes)
	// createCmd.PersistentFlags().BoolVarP(&createForce, "force", "F", false, "Force overwrite file")
}
