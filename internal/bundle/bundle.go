// Package bundle provides shared functionality for generating project bundles.
package bundle

import (
	"fmt"
	"log"
	"path/filepath"
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
	OutputFile     string
	ProjectString  string
	TokenEstimate  string
	ExecutionTime  time.Duration
	FileCount      int
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
	projectString := formatting.CreateProjectString(projectTree, fileContentMap, gitDiffMap)

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
		OutputFile:     opts.OutputFile,
		ProjectString:  projectString,
		TokenEstimate:  fmt.Sprintf("%d - %d tokens", len(projectString)/4, len(projectString)/3),
		ExecutionTime:  elapsed,
		FileCount:      len(filePaths),
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
