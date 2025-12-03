# Project Actions Tool Requirements

## Introduction

The project-actions tool provides security-aware execution of common project development tasks through a project's Makefile and git operations. It enables AI agents to run tests, linters, formatters, and create git commits in a controlled manner without requiring knowledge of project-specific tooling.

## Requirements

### 1. Working Directory Management

**User Story:** As a developer, I want the tool to operate within a specific project directory, so that operations are isolated to my project and don't affect system files.

**Acceptance Criteria:**

1. <a name="1.1"></a>The tool SHALL accept an optional `working_directory` parameter that defaults to the current working directory (`os.Getwd()`)
2. <a name="1.2"></a>The tool SHALL reject working directories that are `/` or typical system directories (`/bin`, `/lib`, `/usr`, `/etc`, `/var`, `/sys`, `/proc`, `/dev`, `/boot`, `/sbin`)
3. <a name="1.3"></a>The tool SHALL verify the working directory is writable by checking that the directory owner is the calling user and the owner write bit is set
4. <a name="1.4"></a>The tool SHALL display the working directory clearly in its output when called
5. <a name="1.5"></a>The tool SHALL validate all file operations occur within the working directory or its subdirectories

### 2. Makefile Integration

**User Story:** As a developer, I want the tool to discover and execute my project's make targets, so that I can run project-specific tasks without manual configuration.

**Acceptance Criteria:**

1. <a name="2.1"></a>The tool SHALL read the Makefile from the working directory using `security.Operations.SafeFileRead()`
2. <a name="2.2"></a>The tool SHALL parse `.PHONY` targets from the Makefile
3. <a name="2.3"></a>The tool SHALL validate target names contain only alphanumeric characters, hyphens, and underscores
4. <a name="2.4"></a>The tool SHALL reject target names containing any other characters
5. <a name="2.5"></a>The tool SHALL execute make targets using `make TARGET_NAME` with no additional parameters
6. <a name="2.6"></a>The tool SHALL NOT scan or validate the commands within `.PHONY` targets

### 3. Makefile Auto-Generation

**User Story:** As a developer working on a project without a Makefile, I want the tool to create a basic Makefile for my language, so that I can immediately use common development tasks.

**Acceptance Criteria:**

1. <a name="3.1"></a>The tool SHALL detect when no Makefile exists in the working directory
2. <a name="3.2"></a>The tool SHALL require a `language` parameter when generating a Makefile (accepted values: `python`, `rust`, `go`, `nodejs`). The tool description SHALL indicate that LLMs may use heuristics to determine the language, but only one language is allowed per Makefile generation
3. <a name="3.3"></a>The tool SHALL generate a minimal Makefile with common targets for the detected language
4. <a name="3.4"></a>The tool SHALL use tab characters (not spaces) for Makefile indentation per Unix conventions
5. <a name="3.5"></a>The tool SHALL generate Makefiles containing only alphanumeric characters, hyphens, underscores, and standard shell commands without pipes, variables, or special characters
6. <a name="3.6"></a>The tool SHALL support Python projects with targets: `default`, `fix` (ruff check --fix), `format` (black), `lint` (ruff check), `test` (pytest)
7. <a name="3.7"></a>The tool SHALL support Rust projects with targets: `default`, `build` (cargo build), `fix` (cargo clippy --fix), `format` (cargo fmt), `lint` (cargo clippy), `test` (cargo test)
8. <a name="3.8"></a>The tool SHALL support Go projects with targets: `default`, `build` (go build), `fix` (go fmt), `format` (go fmt), `lint` (golangci-lint run), `test` (go test ./...)
9. <a name="3.9"></a>The tool SHALL support Node.js projects with targets: `default`, `fix` (npm run lint:fix), `format` (npm run format), `lint` (npm run lint), `test` (npm test)
10. <a name="3.10"></a>The tool SHALL return a diagnostic message instructing the LLM to generate a Makefile when the language parameter is invalid or missing

### 4. Git Add Operations

**User Story:** As a developer, I want to stage files for commit using relative paths, so that I can prepare changes for version control.

**Acceptance Criteria:**

1. <a name="4.1"></a>The tool SHALL accept an array of relative file paths for the `add` operation
2. <a name="4.2"></a>The tool SHALL convert relative paths to absolute paths using `filepath.Clean()` and `filepath.Abs()`
3. <a name="4.3"></a>The tool SHALL validate that resolved absolute paths are within the working directory
4. <a name="4.4"></a>The tool SHALL reject paths that escape the working directory after resolution
5. <a name="4.5"></a>The tool SHALL execute `git add` as a single batch operation for all validated file paths

### 5. Git Commit Operations

**User Story:** As a developer, I want to create commits with custom messages, so that I can document changes in version control.

**Acceptance Criteria:**

1. <a name="5.1"></a>The tool SHALL accept a single commit message parameter (may be multi-line)
2. <a name="5.2"></a>The tool SHALL limit commit messages to 16 KB by default
3. <a name="5.3"></a>The tool SHALL support configuring the commit message size limit via `PROJECT_ACTIONS_MAX_COMMIT_SIZE` environment variable
4. <a name="5.4"></a>The tool SHALL pass commit messages to git via stdin using `git commit --file=-`
5. <a name="5.5"></a>The tool SHALL NOT validate, format, or scan commit message content
6. <a name="5.6"></a>The tool SHALL execute commits in the working directory

### 6. Command Execution and Output

**User Story:** As a developer, I want to see real-time output from commands, so that I can monitor progress and diagnose issues.

**Acceptance Criteria:**

1. <a name="6.1"></a>The tool SHALL stream command output in real-time from both stdout and stderr separately
2. <a name="6.2"></a>The tool SHALL include exit codes in command results
3. <a name="6.3"></a>The tool SHALL include execution time in command results
4. <a name="6.4"></a>The tool SHALL return command output to the caller without modification

### 7. Dry-Run Mode

**User Story:** As a developer, I want to preview commands before execution, so that I can verify operations are safe.

**Acceptance Criteria:**

1. <a name="7.1"></a>The tool SHALL support a `dry_run` parameter for all operations
2. <a name="7.2"></a>The tool SHALL display the exact commands that would be executed in dry-run mode
3. <a name="7.3"></a>The tool SHALL NOT execute any commands when dry-run mode is enabled
4. <a name="7.4"></a>The tool SHALL validate all parameters and paths in dry-run mode

### 8. Tool Availability Validation

**User Story:** As a developer, I want clear error messages when required tools are missing, so that I can install dependencies.

**Acceptance Criteria:**

1. <a name="8.1"></a>The tool SHALL verify `make` and `git` are on the system PATH when the tool initializes
2. <a name="8.2"></a>The tool SHALL emit a clear warning when required tools are not found
3. <a name="8.3"></a>The tool SHALL NOT attempt to install or locate tools beyond checking PATH
4. <a name="8.4"></a>The tool SHALL NOT re-validate tool availability for individual operations after initialization

### 9. Security Integration

**User Story:** As a developer, I want the tool to integrate with the security framework, so that malicious content is detected and blocked.

**Acceptance Criteria:**

1. <a name="9.1"></a>The tool SHALL use `security.Operations` helper functions for file operations
2. <a name="9.2"></a>The tool SHALL use `security.Operations.SafeFileRead()` for reading the Makefile
3. <a name="9.3"></a>The tool SHALL handle `SecurityError` responses appropriately
4. <a name="9.4"></a>The tool SHALL log security warnings when present
5. <a name="9.5"></a>The tool SHALL require the `security` tool to be enabled in `ENABLE_ADDITIONAL_TOOLS`

### 10. Error Handling

**User Story:** As a developer, I want clear error messages when operations fail, so that I can understand and fix issues.

**Acceptance Criteria:**

1. <a name="10.1"></a>The tool SHALL return command output and exit codes for failed operations
2. <a name="10.2"></a>The tool SHALL NOT retry failed operations automatically
3. <a name="10.3"></a>The tool SHALL NOT attempt to handle or interpret command errors
4. <a name="10.4"></a>The tool SHALL return errors immediately without additional processing

### 11. Tool Enablement

**User Story:** As a system administrator, I want the tool disabled by default, so that it follows secure-by-default principles.

**Acceptance Criteria:**

1. <a name="11.1"></a>The tool SHALL be disabled by default
2. <a name="11.2"></a>The tool SHALL be enabled by adding `project_actions` to `ENABLE_ADDITIONAL_TOOLS`
3. <a name="11.3"></a>The tool SHALL document its dependency on the security framework
4. <a name="11.4"></a>The tool SHALL be marked as potentially destructive in MCP annotations


### 12. Makefile Target Discovery

**User Story:** As a developer, I want to see available make targets when the tool initializes, so that I know what actions I can perform.

**Acceptance Criteria:**

1. <a name="12.1"></a>The tool SHALL scan the Makefile for `.PHONY` targets when initialized
2. <a name="12.2"></a>The tool SHALL report available targets as the tool's capabilities
3. <a name="12.3"></a>The tool SHALL auto-generate a Makefile if none exists before reporting capabilities
4. <a name="12.4"></a>The tool SHALL return no actions if the Makefile has no `.PHONY` targets
5. <a name="12.5"></a>The tool SHALL return no actions if the Makefile is malformed
6. <a name="12.6"></a>The tool SHALL NOT dynamically reload the Makefile after initialization

### 13. Git Repository Validation

**User Story:** As a developer, I want clear error messages when git operations fail, so that I can resolve repository issues.

**Acceptance Criteria:**

1. <a name="13.1"></a>The tool SHALL NOT check if the working directory is a git repository
2. <a name="13.2"></a>The tool SHALL pass git command failures directly to the user
3. <a name="13.3"></a>The tool SHALL include a hint to run `git init` or specify another directory when git operations fail
