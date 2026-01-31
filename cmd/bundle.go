// Description: This file contains the generate command which generates a textual representation of the project structure.
package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vossenwout/crev/internal/bundle"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle your project into a single file",
	Long: `Bundle your project into a single file, starting from the directory you are in.
By default common configuration and setup files (ex. .vscode, .venv, package.lock) are ignored as well as non-text extensions like .jpeg, .png, .pdf. 

For more information see: https://crevcli.com/docs

Example usage:
crev bundle
crev bundle --ignore-pre=tests,readme --ignore-ext=.txt 
crev bundle --ignore-pre=tests,readme --include-ext=.go,.py,.js
crev bundle --from-branch=main --to-branch=main
`,
	Args: cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		fromBranch := viper.GetStringSlice("from-branch")
		if len(fromBranch) == 0 {
			log.Fatal("from-branch must be specified")
		}

		toBranch := viper.GetStringSlice("to-branch")
		if len(toBranch) == 0 {
			log.Fatal("to-branch must be specified")
		}

		opts := bundle.Options{
			RootDir:             ".",
			PrefixesToIgnore:    viper.GetStringSlice("ignore-pre"),
			ExtensionsToIgnore:  viper.GetStringSlice("ignore-ext"),
			ExtensionsToInclude: viper.GetStringSlice("include-ext"),
			FromBranch:          fromBranch[0],
			ToBranch:            toBranch[0],
			OutputFile:          "crev-project.txt",
		}

		if err := bundle.GenerateAndLog(opts); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringSlice("ignore-pre", []string{}, "Comma-separated prefixes of file and dir names to ignore. Ex tests,readme")
	generateCmd.Flags().StringSlice("ignore-ext", []string{}, "Comma-separated file extensions to ignore. Ex .txt,.md")
	generateCmd.Flags().StringSlice("include-ext", []string{}, "Comma-separated file extensions to include. Ex .go,.py,.js")
	generateCmd.Flags().StringSlice("from-branch", []string{}, "Branch to compare from. Ex main")
	generateCmd.Flags().StringSlice("to-branch", []string{}, "Branch to compare to. Ex main")

	err := viper.BindPFlag("ignore-pre", generateCmd.Flags().Lookup("ignore-pre"))
	if err != nil {
		log.Fatal(err)
	}

	err = viper.BindPFlag("ignore-ext", generateCmd.Flags().Lookup("ignore-ext"))
	if err != nil {
		log.Fatal(err)
	}

	err = viper.BindPFlag("include-ext", generateCmd.Flags().Lookup("include-ext"))
	if err != nil {
		log.Fatal(err)
	}

	err = viper.BindPFlag("from-branch", generateCmd.Flags().Lookup("from-branch"))
	if err != nil {
		log.Fatal(err)
	}

	err = viper.BindPFlag("to-branch", generateCmd.Flags().Lookup("to-branch"))
	if err != nil {
		log.Fatal(err)
	}
}
