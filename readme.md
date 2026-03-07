# Crev CLI

`crev` bundles your code and runs AI-powered code reviews. It can review a single Bitbucket PR or all PRs where you're assigned, using the Gemini CLI (browser auth, no API key).

## Prerequisites

- **Go** 1.21+ (to build)
- **Gemini CLI** (for `review` / `review-all`): `npm install -g @google/gemini-cli`, then run `gemini` once to sign in
- **Bitbucket Server** credentials (username + app password) for PR commands

## Installation

```bash
git clone https://github.com/vossenwout/crev.git
cd crev
go build -o crev .
# Optional: move to PATH
# mv crev /usr/local/bin/   # or ~/bin, etc.
```

## Configuration

Generate a config file in your project (or home) so you don't pass credentials every time:

```bash
crev init
```

Edit `.crev-config.yaml` and set at least:

- `bitbucket_username` / `bitbucket_password` (or use env `BITBUCKET_USERNAME` / `BITBUCKET_PASSWORD`)
- Optionally `review_prompt: .crev-prompt.md` and other bundle/review options

## Commands

### 1. `crev init`

Creates `.crev-config.yaml` in the current directory with placeholders for Bitbucket credentials, review prompt path, and bundle ignore rules.

```bash
crev init
```

### 2. `crev bundle`

Bundles the current directory into a single text file (e.g. for local inspection or other tools). Requires `--from-branch` and `--to-branch` for git diff.

```bash
crev bundle --from-branch=main --to-branch=feature-branch
crev bundle --from-branch=main --to-branch=main --ignore-pre=tests,readme --include-ext=.go,.py
```

| Flag | Description |
|------|-------------|
| `--from-branch` | Branch to compare from (e.g. `main`) |
| `--to-branch`   | Branch to compare to (e.g. your branch) |
| `--ignore-pre`  | Comma-separated prefixes to ignore |
| `--ignore-ext`  | Comma-separated extensions to ignore |
| `--include-ext` | If set, only these extensions are included |

Output: `crev-project.txt` in the current directory.

### 3. `crev ask`

Sends a prompt to Gemini via the local Gemini CLI (uses your browser login). Use this to confirm Gemini CLI is set up before running reviews.

```bash
crev ask --check
crev ask --prompt "Explain goroutines in Go"
crev ask --prompt "Review this code" --file main.go
crev ask --prompt "Summarize" --file README.md --json
```

| Flag     | Description |
|----------|-------------|
| `--check`  | Verify Gemini CLI is installed and authenticated |
| `--prompt` | Question or instruction to send |
| `--file`   | Optional file to attach as context |
| `--json`   | Return raw JSON from Gemini CLI |

### 4. `crev review` (single PR)

Clones the PR repo, builds a diff bundle, runs the CodeGuardian-style review via Gemini CLI, then posts the review comment and sets PR status (Approve or Needs Work). Skip posting with `--dry-run`.

```bash
crev review --url https://bitbucket.example.com/projects/PROJ/repos/repo/pull-requests/123
crev review --url https://... --username myuser --password my-token --dry-run
crev review --url https://... --prompt .crev-prompt.md
```

| Flag                 | Description |
|----------------------|-------------|
| `--url`              | **Required.** Full Bitbucket Server PR URL |
| `--username`         | Bitbucket username (or set in config) |
| `--password`         | Bitbucket app password / token (or set in config) |
| `--prompt`           | Path to custom review prompt file (default: CodeGuardian built-in) |
| `--include-description` | Include PR title/description in prompt (default: true) |
| `--dry-run`          | Generate review but do not post comment or change PR status |

By default the built-in **CodeGuardian** prompt is used (failure-mode-first Go/backend review). Provide a `.crev-prompt.md` (or other file) with `--prompt` to use your own instructions.

### 5. `crev review-all` (all assigned PRs)

Lists open PRs in the repo where you are a reviewer, then runs the same review pipeline as `crev review` on each. PRs that already have a crev comment are skipped.

```bash
crev review-all --url https://bitbucket.example.com/projects/PROJ/repos/repo/pull-requests/1
crev review-all --url https://... --dry-run
```

Use **any** PR URL from the target repo; the command uses it to identify project and repo, then fetches all open PRs where you're assigned.

| Flag         | Description |
|--------------|-------------|
| `--url`      | **Required.** Any PR URL from the repo to review |
| `--username` | Bitbucket username (or config) |
| `--password` | Bitbucket password (or config) |
| `--prompt`   | Custom review prompt file (default: CodeGuardian) |
| `--include-description` | Include PR title/description (default: true) |
| `--dry-run`  | Do not post comments or update status |

## Quick start (review one PR)

1. Install and authenticate Gemini CLI: `npm i -g @google/gemini-cli`, then `gemini`.
2. Run `crev ask --check` to confirm.
3. Create config: `crev init` and set `bitbucket_username` / `bitbucket_password` in `.crev-config.yaml`.
4. Run a dry run:  
   `crev review --url https://your-bitbucket-server/.../pull-requests/123 --dry-run`
5. If the output looks good, run without `--dry-run` to post the comment and set status.

## Custom review prompt

To use your own review instructions instead of CodeGuardian, create a file (e.g. `.crev-prompt.md`) and pass it with `--prompt`:

```bash
crev review --url https://... --prompt .crev-prompt.md
```

Or set `review_prompt: .crev-prompt.md` in `.crev-config.yaml`.

## Contributing

Contributions are welcome.

## Contact

crevcli@outlook.com
