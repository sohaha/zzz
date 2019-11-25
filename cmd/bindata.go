package cmd

import (
	"errors"
	"github.com/sohaha/zzz/app/bindata"
	"github.com/sohaha/zzz/util"
	
	"github.com/spf13/cobra"
)

var (
	bindataCmdSrc        string
	bindataCmdDest       string
	bindataCmdPkg        string
	bindataCmdTags       string
	bindataCmdForce      bool
	bindataCmdnoMtime    bool
	bindataCmdnoCompress bool
)
var bindataCmd = &cobra.Command{
	Use:        "bindata",
	Aliases:    nil,
	SuggestFor: nil,
	Short:      "Packaging static resource files",
	Long:       "",
	Example: `  zzz bindata -s static -d dest
  zzz bindata --src public/static --dest static
`,
	ValidArgs: nil,
	Args: func(cmd *cobra.Command, args []string) error {
		src, _ := cmd.Flags().GetString("src")
		if src == "" {
			return errors.New("src cannot be empty")
		}
		return nil
	},
	ArgAliases:             nil,
	BashCompletionFunction: "",
	Deprecated:             "",
	Hidden:                 false,
	Annotations:            nil,
	Version:                "",
	PersistentPreRun:       nil,
	PersistentPreRunE:      nil,
	PreRun:                 nil,
	PreRunE:                nil,
	Run: func(cmd *cobra.Command, args []string) {
		if len(cmd.Flags().Args()) > 0 {
			filename := bindata.RunStatic(bindataCmdSrc, bindataCmdDest, bindataCmdTags, bindataCmdPkg, bindataCmdnoMtime, bindataCmdnoCompress, bindataCmdForce)
			util.Log.Successf("Packaged successfully: %s", filename)
			return
		}
		_ = cmd.Help()
	},
	RunE:                       nil,
	PostRun:                    nil,
	PostRunE:                   nil,
	PersistentPostRun:          nil,
	PersistentPostRunE:         nil,
	SilenceErrors:              false,
	SilenceUsage:               false,
	DisableFlagParsing:         false,
	DisableAutoGenTag:          false,
	DisableFlagsInUseLine:      false,
	DisableSuggestions:         false,
	SuggestionsMinimumDistance: 0,
	TraverseChildren:           false,
	FParseErrWhitelist:         cobra.FParseErrWhitelist{},
}

func init() {
	rootCmd.AddCommand(bindataCmd)
	bindataCmd.Flags().StringVarP(&bindataCmdSrc, "src", "s", "", "Static resource directory that needs to be packaged")
	bindataCmd.Flags().StringVarP(&bindataCmdDest, "dest", "d", "./", "Output directory, default to current directory")
	bindataCmd.Flags().StringVarP(&bindataCmdPkg, "pkg", "p", "", "Package name to use in the generated code")
	bindataCmd.Flags().StringVarP(&bindataCmdTags, "tags", "", "", "Optional set of build tags to include")
	bindataCmd.Flags().BoolVarP(&bindataCmdForce, "force", "F", false, "Overwrite the current build static file")
	bindataCmd.Flags().BoolVarP(&bindataCmdnoMtime, "noMtime", "", false, "Do not modify the unix timestamp of all files")
	bindataCmd.Flags().BoolVarP(&bindataCmdnoCompress, "noCompress", "", false, "Do not compress")
}
