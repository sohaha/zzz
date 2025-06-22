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
		Short: "Init new project",
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
					ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
					res, err := zhttp.Get(v, ctx)
					handle := func(res *zhttp.Res) {
						body := res.Bytes()
						zjson.ParseBytes(body).ForEach(func(key, value *zjson.Res) bool {
							name := value.Get("name").String()
							if name == "dev" && strings.HasSuffix(name，"old"){
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
						util.Log.Warn("Timeout, retry")
						res, err = zhttp.Get("https://github.73zls.com/" + v)
						if err == nil {
							handle(res)
						}
					}
				}
				if len(branches) == 0 {
					util.Log.Error("Failed to get the template list, please check your network")
					return
				}
				prompt := promptui.Select{
					Label: "Select Template",
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
			util.Log.Info("Start downloading the template...")
			err := initApp.Clone(dir, name, branch)
			if err != nil {
				util.Log.Fatal(err)
				return
			}
			util.Log.Successf("Init Done: %s\n", dir)
		},
	}
)

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Example = fmt.Sprintf(`  %[1]s %[2]s
  %[1]s %[2]s [dir] [name]`, use, createUes)
	// createCmd.PersistentFlags().BoolVarP(&createForce, "force", "F", false, "Force overwrite file")
}
