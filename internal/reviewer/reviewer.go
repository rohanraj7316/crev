package reviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vossenwout/crev/internal/bundle"
	"github.com/vossenwout/crev/internal/git"
	"github.com/vossenwout/crev/internal/llm"
)

// CrevCommentSignature is embedded in comments posted by crev so we can detect already-reviewed PRs.
const CrevCommentSignature = "<!-- crev-review -->"

// Config configures a single PR review run.
type Config struct {
	GitClient            git.Git
	LLMClient            llm.Client
	PRDetails            *git.PullRequestDetails
	CustomPrompt         string
	UseCodeGuardianDefault bool // if true, use built-in CodeGuardian prompt with structured bundle
	KnowledgeBase        string // optional team conventions/ADRs for CodeGuardian
	IncludePRDesc        bool
	DryRun               bool
	MaxConcurrency       int
}

// Result is the outcome of ReviewPR.
type Result struct {
	PRID    int
	Verdict string // VerdictApprove or VerdictNeedsWork
	Comment string
	Skipped bool
}

// ReviewPR runs the full pipeline: idempotency check, clone, diff bundle, LLM review, post comment, set status.
func ReviewPR(ctx context.Context, cfg Config) (*Result, error) {
	res := &Result{PRID: cfg.PRDetails.ID}

	comments, err := cfg.GitClient.GetPRComments(ctx)
	if err != nil {
		return nil, fmt.Errorf("get PR comments: %w", err)
	}
	for _, c := range comments {
		if strings.Contains(c.Text, CrevCommentSignature) {
			res.Skipped = true
			return res, nil
		}
	}

	crevDir := filepath.Join(os.TempDir(), "crev-pr")
	if err := os.MkdirAll(crevDir, 0755); err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	tempDir, err := os.MkdirTemp(crevDir, "pr-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cloneResult, err := cfg.GitClient.CloneRepository(ctx, tempDir)
	if err != nil {
		return nil, fmt.Errorf("clone repository: %w", err)
	}

	diffOpts := bundle.DiffBundleOptions{
		RootDir:        tempDir,
		FromBranch:     cloneResult.DestBranch,
		ToBranch:       cloneResult.SourceBranch,
		MaxConcurrency: 100,
	}
	if cfg.MaxConcurrency > 0 {
		diffOpts.MaxConcurrency = cfg.MaxConcurrency
	}

	diffResult, err := bundle.GenerateDiffBundle(diffOpts)
	if err != nil {
		return nil, fmt.Errorf("generate diff bundle: %w", err)
	}

	prTitle := cfg.PRDetails.Title
	prDesc := cfg.PRDetails.Description
	var prompt string
	if cfg.UseCodeGuardianDefault {
		prompt = BuildCodeGuardianPrompt(prTitle, prDesc,
			diffResult.ProjectTree, diffResult.FileContext, cfg.KnowledgeBase, diffResult.GitDiff,
			cfg.IncludePRDesc)
	} else {
		prompt = BuildPrompt(cfg.CustomPrompt, prTitle, prDesc, diffResult.ProjectString, cfg.IncludePRDesc)
	}

	reviewText, err := cfg.LLMClient.GenerateText(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generate: %w", err)
	}

	reviewText = strings.TrimSpace(reviewText)
	if reviewText == "" {
		reviewText = "No review generated."
	}

	res.Verdict = ParseVerdict(reviewText)
	commentBody := strings.TrimSpace(StripVerdictFromComment(reviewText))

	summary, issueBlocks := ParseReviewBody(commentBody)
	if summary == "" {
		summary = commentBody
	}
	summaryComment := summary + "\n\n" + CrevCommentSignature

	if !cfg.DryRun {
		if err := cfg.GitClient.PostComment(ctx, summaryComment); err != nil {
			return nil, fmt.Errorf("post summary comment: %w", err)
		}
		for _, block := range issueBlocks {
			if strings.TrimSpace(block) == "" {
				continue
			}
			if err := cfg.GitClient.PostComment(ctx, block); err != nil {
				return nil, fmt.Errorf("post issue comment: %w", err)
			}
		}
		if res.Verdict == VerdictApprove {
			if err := cfg.GitClient.Approve(ctx); err != nil {
				return nil, fmt.Errorf("approve PR: %w", err)
			}
		} else {
			if err := cfg.GitClient.RequestChanges(ctx); err != nil {
				return nil, fmt.Errorf("request changes: %w", err)
			}
		}
	}

	// Result.Comment is the full review text (summary + issues) for logging
	res.Comment = summaryComment
	for _, block := range issueBlocks {
		res.Comment += "\n\n---\n\n" + block
	}
	return res, nil
}
