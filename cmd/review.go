package cmd

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vossenwout/crev/internal/git"
	"github.com/vossenwout/crev/internal/llm"
	"github.com/vossenwout/crev/internal/reviewer"
)

// defaultReviewPrompt is used only when a custom prompt file is provided but empty; otherwise CodeGuardian is used.
const defaultReviewPrompt = `You are a senior software engineer performing a code review. Review the following project snapshot (directory structure, file contents, and git diffs where present). Provide a concise, actionable code review in markdown. Focus on: correctness, security, maintainability, performance, and style. Be constructive and specific.`

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Auto-review a single Bitbucket PR with AI",
	Long: `Clone the PR repo, generate a diff bundle, send it to Gemini CLI for review, then post the review comment and set PR status (Approve or Needs Work).

By default uses the built-in CodeGuardian prompt (failure-mode-first Go/backend review). Use --prompt to supply a custom .crev-prompt.md file instead.
Requires Bitbucket credentials and the Gemini CLI to be installed and authenticated (crev ask --check).

Examples:
  crev review --url https://bitbucket.example.com/projects/PROJ/repos/repo/pull-requests/123
  crev review --url https://... --dry-run
  crev review --url https://... --prompt .crev-prompt.md
`,
	Run: func(c *cobra.Command, args []string) {
		prURL, _ := c.Flags().GetString("url")
		if prURL == "" {
			prURL = viper.GetString("url")
		}
		username, _ := c.Flags().GetString("username")
		if username == "" {
			username = viper.GetString("bitbucket_username")
		}
		password, _ := c.Flags().GetString("password")
		if password == "" {
			password = viper.GetString("bitbucket_password")
		}
		promptPath, _ := c.Flags().GetString("prompt")
		if promptPath == "" {
			promptPath = viper.GetString("review_prompt")
		}
		includeDesc := true
		if c.Flags().Changed("include-description") {
			includeDesc, _ = c.Flags().GetBool("include-description")
		} else if viper.IsSet("include_pr_description") {
			includeDesc = viper.GetBool("include_pr_description")
		}
		dryRun, _ := c.Flags().GetBool("dry-run")
		if !c.Flags().Changed("dry-run") && viper.IsSet("dry_run") {
			dryRun = viper.GetBool("dry_run")
		}

		if prURL == "" {
			log.Fatal("--url is required")
		}
		if username == "" {
			log.Fatal("bitbucket_username is required (flag --username or config)")
		}
		if password == "" {
			log.Fatal("bitbucket_password is required (flag --password or config)")
		}

		useCodeGuardian := true
		customPrompt := defaultReviewPrompt
		if promptPath != "" {
			data, err := os.ReadFile(promptPath)
			if err == nil {
				customPrompt = string(data)
				useCodeGuardian = false
			} else if !os.IsNotExist(err) {
				log.Fatalf("read prompt file %s: %v", promptPath, err)
			}
		}

		gitClient, err := git.NewGitBitbucketFromURL(prURL, username, password)
		if err != nil {
			log.Fatalf("Bitbucket client: %v", err)
		}

		llmClient, err := llm.NewGeminiCLI()
		if err != nil {
			log.Fatalf("Gemini CLI: %v (run 'crev ask --check')", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		prDetails, err := gitClient.GetPullRequestDetails(ctx)
		if err != nil {
			log.Fatalf("get PR details: %v", err)
		}

		cfg := reviewer.Config{
			GitClient:              gitClient,
			LLMClient:              llmClient,
			PRDetails:              prDetails,
			CustomPrompt:           customPrompt,
			UseCodeGuardianDefault: useCodeGuardian,
			IncludePRDesc:          includeDesc,
			DryRun:                 dryRun,
			MaxConcurrency:         100,
		}

		result, err := reviewer.ReviewPR(ctx, cfg)
		if err != nil {
			log.Fatalf("review: %v", err)
		}

		if result.Skipped {
			log.Printf("PR #%d already reviewed by crev, skipping.", result.PRID)
			return
		}

		log.Printf("PR #%d reviewed. Verdict: %s", result.PRID, result.Verdict)
		if dryRun {
			log.Printf("[dry-run] Would have posted comment and set status to %s", result.Verdict)
		}
	},
}

func init() {
	rootCmd.AddCommand(reviewCmd)

	reviewCmd.Flags().String("url", "", "Bitbucket pull request URL")
	reviewCmd.Flags().String("username", "", "Bitbucket username")
	reviewCmd.Flags().String("password", "", "Bitbucket password")
	reviewCmd.Flags().String("prompt", ".crev-prompt.md", "Path to custom review prompt file")
	reviewCmd.Flags().Bool("include-description", true, "Include PR title and description in the review prompt")
	reviewCmd.Flags().Bool("dry-run", false, "Generate review but do not post comment or update PR status")

	_ = viper.BindPFlag("url", reviewCmd.Flags().Lookup("url"))
	_ = viper.BindPFlag("bitbucket_username", reviewCmd.Flags().Lookup("username"))
	_ = viper.BindPFlag("bitbucket_password", reviewCmd.Flags().Lookup("password"))
	_ = viper.BindPFlag("review_prompt", reviewCmd.Flags().Lookup("prompt"))
	_ = viper.BindPFlag("include_pr_description", reviewCmd.Flags().Lookup("include-description"))
	_ = viper.BindPFlag("dry_run", reviewCmd.Flags().Lookup("dry-run"))
}
