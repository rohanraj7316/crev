// Description: This file implements the "init" command, which generates a default configuration file in the current directory.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Define a default template configuration
var defaultConfig = []byte(`# Configuration for the crev tool

# Bitbucket credentials (for crev review and crev review-all)
bitbucket_username: # ex. your-username
bitbucket_password: # ex. app password or token

# Path to custom review prompt file (markdown)
review_prompt: .crev-prompt.md

# Include PR title and description in the AI review prompt
include_pr_description: true

# Optional: CREV API key if using API-based LLM instead of Gemini CLI
crev_api_key:

# Bundle command: prefixes and extensions to ignore/include
ignore-pre: # ex. [tests, readme.md, scripts]
ignore-ext: # ex. [.go, .py, .js]
include-ext: # ex. [.go, .py, .js]
`)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a default configuration file",
	Long: `Generates a default configuration file (.crev-config.yaml) in the current directory.

The configuration file includes:
- Bitbucket credentials (for crev review and crev review-all)
- Path to custom review prompt file
- File and directory ignore patterns for the bundle command
- File extensions to include when generating the project overview

You can modify this file as needed to suit your project's structure.
`,
	Run: func(cmd *cobra.Command, args []string) {
		configFileName := ".crev-config.yaml"

		// Check if the config file already exists
		if viper.ConfigFileUsed() != "" {
			fmt.Println("Config file already exists at ", viper.ConfigFileUsed())
			os.Exit(1)
		}

		// Write the default config using Viper
		err := os.WriteFile(configFileName, defaultConfig, 0644)
		if err != nil {
			fmt.Println("Unable to write config file: ", err)
			os.Exit(1)
		}

		// Inform the user
		fmt.Println("Config file created at:", configFileName)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
