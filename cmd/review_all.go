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

var reviewAllCmd = &cobra.Command{
	Use:   "review-all",
	Short: "Auto-review all open PRs where you are assigned as reviewer",
	Long: `List open pull requests for the repo where you are a reviewer, then run the same review pipeline as 'crev review' on each. Skips PRs that crev has already commented on.

Requires Bitbucket credentials and Gemini CLI (crev ask --check). Use --url with any PR URL from the target repository to identify the repo.

Examples:
  crev review-all --url https://bitbucket.example.com/projects/PROJ/repos/repo/pull-requests/1
  crev review-all --url https://... --dry-run
`,
	Run: func(c *cobra.Command, args []string) {
		prURL, _ := c.Flags().GetString("url")
		if prURL == "" {
			prURL = viper.GetString("review_all_url")
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
			log.Fatal("--url is required (use any PR URL from the repo to review)")
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

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		prs, err := gitClient.ListOpenPullRequests(ctx)
		if err != nil {
			log.Fatalf("list open PRs: %v", err)
		}

		log.Printf("Found %d open PR(s) where you are assigned.", len(prs))
		if len(prs) == 0 {
			return
		}

		var reviewed, skipped, errors int
		for _, pr := range prs {
			scoped := gitClient.WithPRID(pr.ID)
			cfg := reviewer.Config{
				GitClient:              scoped,
				LLMClient:              llmClient,
				PRDetails:              pr,
				CustomPrompt:           customPrompt,
				UseCodeGuardianDefault: useCodeGuardian,
				IncludePRDesc:          includeDesc,
				DryRun:                 dryRun,
				MaxConcurrency:         100,
			}

			result, err := reviewer.ReviewPR(ctx, cfg)
			if err != nil {
				log.Printf("PR #%d error: %v", pr.ID, err)
				errors++
				continue
			}
			if result.Skipped {
				log.Printf("PR #%d skipped (already reviewed).", pr.ID)
				skipped++
				continue
			}
			log.Printf("PR #%d reviewed. Verdict: %s", pr.ID, result.Verdict)
			reviewed++
		}

		log.Printf("Done. Reviewed: %d, Skipped: %d, Errors: %d", reviewed, skipped, errors)
		if dryRun && reviewed > 0 {
			log.Printf("[dry-run] No comments or status changes were made.")
		}
	},
}

func init() {
	rootCmd.AddCommand(reviewAllCmd)

	reviewAllCmd.Flags().String("url", "", "Any Bitbucket PR URL from the repo to review")
	reviewAllCmd.Flags().String("username", "", "Bitbucket username")
	reviewAllCmd.Flags().String("password", "", "Bitbucket password")
	reviewAllCmd.Flags().String("prompt", ".crev-prompt.md", "Path to custom review prompt file")
	reviewAllCmd.Flags().Bool("include-description", true, "Include PR title and description in the review prompt")
	reviewAllCmd.Flags().Bool("dry-run", false, "Generate reviews but do not post comments or update PR status")

	_ = viper.BindPFlag("review_all_url", reviewAllCmd.Flags().Lookup("url"))
	_ = viper.BindPFlag("bitbucket_username", reviewAllCmd.Flags().Lookup("username"))
	_ = viper.BindPFlag("bitbucket_password", reviewAllCmd.Flags().Lookup("password"))
	_ = viper.BindPFlag("review_prompt", reviewAllCmd.Flags().Lookup("prompt"))
	_ = viper.BindPFlag("include_pr_description", reviewAllCmd.Flags().Lookup("include-description"))
	_ = viper.BindPFlag("dry_run", reviewAllCmd.Flags().Lookup("dry-run"))
}
