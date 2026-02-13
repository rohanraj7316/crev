package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vossenwout/crev/internal/bitbucket"
	"github.com/vossenwout/crev/internal/bundle"
)

// prCmd represents the pr command
var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Fetch and bundle code from a Bitbucket pull request",
	Long: `Fetch pull request details and code changes from Bitbucket and bundle them for review.

This command requires Bitbucket credentials to be configured:
- BITBUCKET_USERNAME and BITBUCKET_PASSWORD (or app password) environment variables
- Or set bitbucket_username and bitbucket_password in .crev-config.yaml
- Or use --username and --password flags

For Bitbucket Cloud, the base URL is: https://api.bitbucket.org/2.0
For Bitbucket Server, use: https://your-server.com/rest/api/1.0

Example usage:
  crev pr --url https://aslbitbucket.asldt.in/projects/BH/repos/crobat/pull-requests/50/overview
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get configuration
		prURL := viper.GetString("url")
		username := viper.GetString("username")
		password := viper.GetString("password")
		outputFile := viper.GetString("output")

		// Validate required fields
		if prURL == "" {
			log.Fatal("pr-url is required. Use --url flag or set bitbucket_pr_url in config")
		}

		if username == "" {
			log.Fatal("username is required. Use --username flag or set bitbucket_username in config")
		}

		if password == "" {
			log.Fatal("password is required. Use --password flag or set bitbucket_password in config")
		}

		// Set default output file
		if outputFile == "" {
			outputFile = "crev-project.txt"
		}

		git, err := bitbucket.NewGitBitbucketFromURL(prURL, username, password)
		if err != nil {
			log.Fatalf("Failed to create Bitbucket client: %v", err)
		}

		log.Printf("Clone URL: %s", git.GetCloneURL())

		// Create new path in /Users/unio/Documents for cloning
		crevDir := filepath.Join("/Users/unio/Documents", "crev-pr")
		if err := os.MkdirAll(crevDir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", crevDir, err)
		}
		// Use a unique subdirectory inside /Users/unio/Documents/crev-pr
		tempDir, err := os.MkdirTemp(crevDir, "pr-*")
		if err != nil {
			log.Fatalf("Failed to create temp directory in %s: %v", crevDir, err)
		}
		defer os.RemoveAll(tempDir) // Cleanup after we're done

		log.Printf("Cloning repository to %s ...", tempDir)

		cloneResult, err := git.CloneRepository(context.Background(), tempDir)
		if err != nil {
			log.Fatalf("Failed to clone repository: %v", err)
		}

		log.Printf("Clone successful!")
		log.Printf("  Source Branch: %s", cloneResult.SourceBranch)
		log.Printf("  Destination Branch: %s", cloneResult.DestBranch)

		// Generate bundle from the cloned repository
		log.Printf("Generating project bundle...")

		// Generate output file inside tempDir first
		tempOutputFile := filepath.Join(tempDir, outputFile)

		opts := bundle.Options{
			RootDir:    tempDir,
			FromBranch: cloneResult.SourceBranch, // Compare from destination (target) branch
			ToBranch:   cloneResult.DestBranch,   // Compare to source (PR) branch
			OutputFile: tempOutputFile,
		}

		if err := bundle.GenerateAndLog(opts); err != nil {
			log.Fatalf("Failed to generate bundle: %v", err)
		}

		// Prompt user for review comment
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("Enter your review comment in Markdown format.")
		fmt.Println("Type ':wq' on a new line when done.")
		fmt.Println(strings.Repeat("=", 60))

		comment := readMultiLineInput(":wq")

		if strings.TrimSpace(comment) == "" {
			log.Println("No comment provided. Skipping.")
			return
		}

		// Show preview
		fmt.Println("\n" + strings.Repeat("-", 40))
		fmt.Println("Comment Preview:")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println(comment)
		fmt.Println(strings.Repeat("-", 40))

		// Ask user if they want to skip posting
		fmt.Print("\nSkip posting this comment? (y/n): ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		response := strings.TrimSpace(strings.ToLower(scanner.Text()))

		if response == "y" || response == "yes" {
			log.Println("Skipping comment posting.")
			return
		}

		// Post comment to Bitbucket PR
		log.Println("Posting comment to pull request...")

		if err := git.PostComment(context.Background(), comment); err != nil {
			log.Fatalf("Failed to post comment: %v", err)
		}

		log.Println("Comment posted successfully!")

		// Set PR status to "Needs Work" (Request Changes)
		log.Println("Setting PR status to 'Needs Work'...")

		if err := git.RequestChanges(context.Background()); err != nil {
			log.Printf("Warning: Failed to set PR status: %v", err)
		} else {
			log.Println("PR status set to 'Needs Work' (Request Changes)")
		}
	},
}

// readMultiLineInput reads multiple lines from stdin until terminator is entered
func readMultiLineInput(terminator string) string {
	scanner := bufio.NewScanner(os.Stdin)
	var lines []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == terminator {
			break
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
		return ""
	}

	return strings.Join(lines, "\n")
}

func init() {
	rootCmd.AddCommand(prCmd)

	// Command flags
	prCmd.Flags().String("url", "", "Bitbucket pull request URL")
	prCmd.Flags().String("username", "", "Bitbucket username")
	prCmd.Flags().String("password", "", "Bitbucket password")
	prCmd.Flags().String("output", "crev-project.txt", "Output file name")

	// Bind flags to viper
	viper.BindPFlag("url", prCmd.Flags().Lookup("url"))
	viper.BindPFlag("username", prCmd.Flags().Lookup("username"))
	viper.BindPFlag("password", prCmd.Flags().Lookup("password"))
	viper.BindPFlag("output", prCmd.Flags().Lookup("output"))
}
