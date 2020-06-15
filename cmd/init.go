package cmd

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zhttp"
	"github.com/sohaha/zlsgo/zjson"
	"github.com/spf13/cobra"

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
				res, err := zhttp.Get("https://api.github.com/repos/sohaha/zlsgo-app/branches")
				if err == nil {
					body := res.Bytes()
					var branches []string
					zjson.ParseBytes(body).ForEach(func(key, value zjson.Res) bool {
						name := value.Get("name").String()
						if name == "master" {
							return true
						}
						branches = append(branches, "zlsgo-app/"+name)
						return true
					})
					prompt := promptui.Select{
						Label: "Select Template",
						Items: branches,
					}
					_, result, err := prompt.Run()
					if err == nil {
						tmp = "sohaha/" + result
					}
				}
			}

			temples := strings.Split(tmp, "/")
			branch := "master"
			name := tmp
			if len(temples) > 2 {
				branch = temples[2]
				name = strings.Join(temples[:2], "/")
			}

			dir := ""
			if argsL > 0 {
				dir = zfile.RealPath(args[0])
			} else {
				dir = "."
				if len(temples) >= 2 {
					dir = temples[1]
				}
				dir = zfile.RealPath(dir)
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
