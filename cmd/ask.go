package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vossenwout/crev/internal/llm"
)

var askCmd = &cobra.Command{
	Use:   "ask",
	Short: "Send a prompt to Gemini via the local Gemini CLI",
	Long: `Send a prompt to Gemini using the locally installed Gemini CLI (browser-based auth, no API key needed).

Prerequisites:
  1. Install Gemini CLI: npm install -g @google/gemini-cli
  2. Authenticate once: gemini (follow browser OAuth flow)

Examples:
  crev ask --check
  crev ask --prompt "Explain goroutines in Go"
  crev ask --prompt "Review this code" --file main.go
  crev ask --prompt "Review this code" --file main.go --json
`,
	Run: func(cmd *cobra.Command, args []string) {
		checkOnly, _ := cmd.Flags().GetBool("check")
		prompt, _ := cmd.Flags().GetString("prompt")
		filePath, _ := cmd.Flags().GetString("file")
		jsonOutput, _ := cmd.Flags().GetBool("json")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if checkOnly {
			runCheck(ctx)
			return
		}

		if prompt == "" {
			log.Fatal("--prompt is required (or use --check to verify setup)")
		}

		client, err := llm.NewGeminiCLI()
		if err != nil {
			log.Fatalf("Setup error: %v", err)
		}

		cliClient := client.(*llm.GeminiCLIClient)

		var fileContent string
		if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				log.Fatalf("Failed to read file %s: %v", filePath, err)
			}
			fileContent = string(data)
		}

		if jsonOutput {
			raw, err := cliClient.GenerateJSON(ctx, prompt, fileContent)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			fmt.Println(string(raw))
			return
		}

		var result string
		if fileContent != "" {
			result, err = cliClient.GenerateTextWithFile(ctx, prompt, fileContent)
		} else {
			result, err = client.GenerateText(ctx, prompt)
		}
		if err != nil {
			log.Fatalf("Error: %v", err)
		}

		fmt.Println(result)
	},
}

func runCheck(ctx context.Context) {
	fmt.Println("Checking Gemini CLI setup...")

	fmt.Print("  Binary: ")
	path, err := llm.FindGeminiBinary()
	if err != nil {
		fmt.Println("NOT FOUND")
		fmt.Println("  Install: npm install -g @google/gemini-cli")
		os.Exit(1)
	}
	fmt.Println(path)

	fmt.Print("  Auth:   ")
	if err := llm.CheckAuth(ctx); err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  %v\n", err)
		fmt.Println("  Run 'gemini' interactively to authenticate via browser")
		os.Exit(1)
	}
	fmt.Println("OK")

	fmt.Println("\nGemini CLI is ready.")
}

func init() {
	rootCmd.AddCommand(askCmd)

	askCmd.Flags().Bool("check", false, "Verify Gemini CLI binary and auth status")
	askCmd.Flags().String("prompt", "", "Prompt to send to Gemini")
	askCmd.Flags().String("file", "", "File to include as context (piped via stdin to Gemini CLI)")
	askCmd.Flags().Bool("json", false, "Return raw JSON output from Gemini CLI")
}
