// Package bundle provides shared functionality for generating project bundles.
package bundle

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vossenwout/crev/internal/files"
	"github.com/vossenwout/crev/internal/formatting"
)

// StandardPrefixesToIgnore contains common prefixes to exclude from bundling
var StandardPrefixesToIgnore = []string{
	// ignore .git, .idea, .vscode, etc.
	".",
	// ignore crev specific files
	"crev",
	// ignore go.mod, go.sum, etc.
	"go",
	"license",
	// readme
	"readme",
	"README",
	// poetry
	"pyproject.toml",
	"poetry.lock",
	"venv",
	// output files
	"build",
	"dist",
	"out",
	"target",
	"bin",
	// javascript
	"node_modules",
	"coverage",
	"public",
	"static",
	"Thumbs.db",
	"package",
	"yarn.lock",
	"package",
	"tsconfig",
	// next.js
	"next.config",
	"next-env",
	// python
	"__pycache__",
	"logs",
	// java
	"gradle",
	// c++
	"CMakeLists",
	// ruby
	"vendor",
	"Gemfile",
	// php
	"composer",
	// tailwind
	"tailwind",
	"postcss",
}

// StandardExtensionsToIgnore contains common file extensions to exclude from bundling
var StandardExtensionsToIgnore = []string{
	".jpeg",
	".jpg",
	".png",
	".gif",
	".pdf",
	".svg",
	".ico",
	".woff",
	".woff2",
	".eot",
	".ttf",
	".otf",
}

// Options configures how the bundle is generated
type Options struct {
	RootDir             string   // Root directory to bundle
	PrefixesToIgnore    []string // Additional prefixes to ignore
	ExtensionsToIgnore  []string // Additional extensions to ignore
	ExtensionsToInclude []string // Extensions to include (if set, only these are included)
	FromBranch          string   // Git branch to compare from (optional)
	ToBranch            string   // Git branch to compare to (optional)
	OutputFile          string   // Output file path
	MaxConcurrency      int      // Max concurrent file reads
}

// Result contains the result of a bundle generation
type Result struct {
	OutputFile    string
	ProjectString string
	TokenEstimate string
	ExecutionTime time.Duration
	FileCount     int
	// Structured fields (only set by GenerateDiffBundle) for prompt templates that expect separate sections
	ProjectTree string
	FileContext string
	GitDiff     string
}

// DefaultOptions returns sensible defaults for bundling
func DefaultOptions() Options {
	return Options{
		RootDir:        ".",
		OutputFile:     "crev-project.txt",
		MaxConcurrency: 100,
	}
}

// Generate creates a project bundle from the specified directory
func Generate(opts Options) (*Result, error) {
	start := time.Now()

	// Apply defaults
	if opts.RootDir == "" {
		opts.RootDir = "."
	}
	if opts.OutputFile == "" {
		opts.OutputFile = "crev-project.txt"
	}
	if opts.MaxConcurrency == 0 {
		opts.MaxConcurrency = 100
	}

	// Combine user-specified prefixes with standard ones
	prefixesToIgnore := append(opts.PrefixesToIgnore, StandardPrefixesToIgnore...)
	extensionsToIgnore := append(opts.ExtensionsToIgnore, StandardExtensionsToIgnore...)

	// Get all file paths from the root directory
	filePaths, err := files.GetAllFilePaths(
		opts.RootDir,
		prefixesToIgnore,
		opts.ExtensionsToInclude,
		extensionsToIgnore,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get file paths: %w", err)
	}

	// Generate the project tree
	projectTree := formatting.GeneratePathTree(filePaths)

	// Get the content of all files
	fileContentMap, err := files.GetContentMapOfFiles(filePaths, opts.MaxConcurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to get file contents: %w", err)
	}

	// Get git diff if branches are specified
	var gitDiffMap map[string]string
	if opts.FromBranch != "" && opts.ToBranch != "" {
		gitDiffMap, err = files.GetGitDiffOfContentMapOfFiles(opts.RootDir, opts.FromBranch, opts.ToBranch, opts.MaxConcurrency)
		if err != nil {
			return nil, fmt.Errorf("failed to get git diff: %w", err)
		}
	}

	// Create the project string
	projectString := formatting.CreateProjectString(opts.RootDir, projectTree, fileContentMap, gitDiffMap)

	// Ensure output directory exists
	outputDir := filepath.Dir(opts.OutputFile)
	if outputDir != "." && outputDir != "" {
		if err := files.EnsureDir(outputDir); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Save the project string to a file
	if err := files.SaveStringToFile(projectString, opts.OutputFile); err != nil {
		return nil, fmt.Errorf("failed to save bundle: %w", err)
	}

	elapsed := time.Since(start)

	return &Result{
		OutputFile:    opts.OutputFile,
		ProjectString: projectString,
		TokenEstimate: fmt.Sprintf("%d - %d tokens", len(projectString)/4, len(projectString)/3),
		ExecutionTime: elapsed,
		FileCount:     len(filePaths),
	}, nil
}

// GenerateAndLog generates a bundle and logs the results
func GenerateAndLog(opts Options) error {
	result, err := Generate(opts)
	if err != nil {
		return err
	}

	log.Printf("Project overview successfully saved to: %s", result.OutputFile)
	log.Printf("Files bundled: %d", result.FileCount)
	log.Printf("Estimated token count: %s", result.TokenEstimate)
	log.Printf("Execution time: %s", result.ExecutionTime)

	return nil
}

// DiffBundleOptions configures diff-only bundle generation (changed files + their content + diff).
type DiffBundleOptions struct {
	RootDir        string
	FromBranch     string // branch to compare from (e.g. destination/target)
	ToBranch       string // branch to compare to (e.g. source/PR branch)
	MaxConcurrency int
}

// GenerateDiffBundle creates a project string containing only changed files between two branches:
// full content of each changed file plus the git diff. Does not write to disk.
func GenerateDiffBundle(opts DiffBundleOptions) (*Result, error) {
	start := time.Now()

	if opts.RootDir == "" {
		opts.RootDir = "."
	}
	if opts.MaxConcurrency == 0 {
		opts.MaxConcurrency = 100
	}

	gitDiffMap, err := files.GetGitDiffOfContentMapOfFiles(opts.RootDir, opts.FromBranch, opts.ToBranch, opts.MaxConcurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to get git diff: %w", err)
	}

	if len(gitDiffMap) == 0 {
		return &Result{
			ProjectString: "",
			TokenEstimate: "0 tokens",
			ExecutionTime: time.Since(start),
			FileCount:     0,
		}, nil
	}

	// Build full paths for changed files (git diff returns paths relative to workDir)
	var fullPaths []string
	for relPath := range gitDiffMap {
		fullPaths = append(fullPaths, filepath.Join(opts.RootDir, relPath))
	}

	fileContentMap, err := files.GetContentMapOfFiles(fullPaths, opts.MaxConcurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to get file contents: %w", err)
	}

	projectTree := formatting.GeneratePathTree(fullPaths)
	projectString := formatting.CreateProjectString(opts.RootDir, projectTree, fileContentMap, gitDiffMap)

	fileContext := buildFileContext(opts.RootDir, fullPaths, fileContentMap)
	gitDiffStr := buildGitDiffString(gitDiffMap)

	elapsed := time.Since(start)

	return &Result{
		ProjectString: projectString,
		ProjectTree:   projectTree,
		FileContext:   fileContext,
		GitDiff:       gitDiffStr,
		TokenEstimate: fmt.Sprintf("%d - %d tokens", len(projectString)/4, len(projectString)/3),
		ExecutionTime: elapsed,
		FileCount:     len(fullPaths),
	}, nil
}

func buildFileContext(rootDir string, fullPaths []string, fileContentMap map[string]string) string {
	var b strings.Builder
	for _, fullPath := range fullPaths {
		rel := fullPath
		if rootDir != "" {
			if r, err := filepath.Rel(rootDir, fullPath); err == nil {
				rel = r
			}
		}
		content := fileContentMap[fullPath]
		b.WriteString("--- " + rel + "\n")
		b.WriteString(content)
		b.WriteString("\n\n")
	}
	return b.String()
}

func buildGitDiffString(gitDiffMap map[string]string) string {
	paths := make([]string, 0, len(gitDiffMap))
	for p := range gitDiffMap {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		b.WriteString("--- " + p + "\n")
		b.WriteString(gitDiffMap[p])
		b.WriteString("\n\n")
	}
	return b.String()
}
