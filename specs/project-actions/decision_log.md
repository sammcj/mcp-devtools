# Decision Log - Project Actions Tool

## Decision 1: Feature Name
**Date:** 2025-01-XX
**Decision:** Use `project-actions` as the feature name
**Rationale:** Clearly describes the tool's purpose of performing project-level actions (tests, linting, formatting, git commits) in a controlled manner
**Alternatives Considered:** `dev-actions`, `project-tasks`

## Decision 2: Makefile-Only Approach
**Date:** 2025-01-XX
**Decision:** Support tests, linters, and formatters only via Makefile integration
**Rationale:**
- Provides a single, consistent interface regardless of project language or tooling
- Avoids tool-specific detection and configuration complexity
- Leverages existing project build infrastructure
- Reduces security surface by limiting to make target names only
**Alternatives Considered:** Direct tool detection (pytest, go test, npm test, etc.)

## Decision 3: Target Name Validation
**Date:** 2025-01-XX
**Decision:** Restrict make target names to alphanumeric, hyphen, and underscore only
**Rationale:**
- Minimizes command injection attack surface
- Simple validation rule that covers 99% of legitimate use cases
- No need to scan target commands since we only execute via `make TARGET_NAME`
**Alternatives Considered:** More permissive validation with command scanning

## Decision 4: Working Directory Security
**Date:** 2025-01-XX
**Decision:** Accept optional `working_directory` parameter, default to `os.Getwd()`, block system directories
**Rationale:**
- Aligns with filesystem tool's security model
- Prevents accidental system-wide operations
- Simpler than environment variable configuration
- Clear validation rules for safe directories
**Alternatives Considered:** Environment variable like `FILESYSTEM_TOOL_ALLOWED_DIRS`

## Decision 5: Git Commit Message via Stdin
**Date:** 2025-01-XX
**Decision:** Pass commit messages via `git commit --file=-` on stdin
**Rationale:**
- Prevents shell interpretation of commit message content
- Eliminates command injection risk from special characters in messages
- Standard git practice for programmatic commits
**Alternatives Considered:** Temporary file, command-line argument (rejected due to security)

## Decision 6: No Commit Message Validation
**Date:** 2025-01-XX
**Decision:** Do not scan or validate commit message content
**Rationale:**
- Commit message validation is the role of git hooks
- Out of scope for this tool
- Allows maximum flexibility for commit message formats
**Alternatives Considered:** Conventional commits validation, sensitive data scanning

## Decision 7: Generous Commit Message Size Limit
**Date:** 2025-01-XX
**Decision:** Default 16 KB commit message limit, configurable via environment variable
**Rationale:**
- Accommodates detailed commit messages and multi-paragraph descriptions
- Prevents abuse while being practical
- Tunable for different project needs
**Alternatives Considered:** 4 KB (too restrictive), unlimited (potential abuse)

## Decision 8: Auto-Generate Makefiles
**Date:** 2025-01-XX
**Decision:** Automatically create basic Makefiles when missing, based on language detection
**Rationale:**
- Reduces friction for projects without existing Makefiles
- Provides immediate value without manual setup
- Generated content is controlled and safe
**Alternatives Considered:** Require manual Makefile creation, return error when missing

## Decision 9: Real-Time Output Streaming
**Date:** 2025-01-XX
**Decision:** Stream command output in real-time from stdout and stderr
**Rationale:**
- Provides immediate feedback for long-running operations
- Matches developer expectations for command execution
- Enables monitoring of test/lint progress
**Alternatives Considered:** Buffered output (poor UX for long operations)

## Decision 10: Minimal Error Handling
**Date:** 2025-01-XX
**Decision:** Return command output and exit codes without retry or interpretation
**Rationale:**
- Tool acts as a thin wrapper around make/git
- Caller (AI agent) can interpret errors and decide on actions
- Simpler implementation with fewer edge cases
**Alternatives Considered:** Automatic retry, error interpretation

## Decision 11: Security Helper Functions
**Date:** 2025-01-XX
**Decision:** Use `security.Operations` helper functions for file operations
**Rationale:**
- Consistent with other tools in the codebase
- Automatic security integration
- Reduced boilerplate (80-90% reduction)
- Content integrity preservation
**Alternatives Considered:** Manual security integration

## Decision 12: Disabled by Default
**Date:** 2025-01-XX
**Decision:** Tool disabled by default, enabled via `ENABLE_ADDITIONAL_TOOLS`
**Rationale:**
- Follows secure-by-default principle
- Potentially destructive operations (git commits, code modification via formatters)
- Requires explicit opt-in from users
**Alternatives Considered:** Enabled by default (rejected for security)


## Decision 13: Multi-Language Support
**Date:** 2025-01-XX
**Decision:** Support Python, Rust, Go, and Node.js with language-specific common conventions
**Rationale:**
- Covers the most common languages in modern development
- Uses standard tooling conventions for each ecosystem
- Provides immediate value without configuration
- Build targets for compiled languages (Rust, Go) enable full development workflow
**Language-Specific Conventions:**
- **Python:** ruff (fix/lint), black (format), pytest (test)
- **Rust:** cargo clippy (fix/lint), cargo fmt (format), cargo test, cargo build
- **Go:** go fmt (fix/format), golangci-lint (lint), go test, go build
- **Node.js:** eslint (fix/lint), prettier (format), npm test
**Alternatives Considered:** Single language support, user-provided configuration


## Decision 14: Language Parameter Required
**Date:** 2025-01-XX
**Decision:** Require explicit `language` parameter for Makefile generation instead of auto-detection
**Rationale:**
- Eliminates ambiguity in multi-language projects
- Simpler implementation without complex detection heuristics
- LLMs can use their own heuristics to determine language before calling tool
- Clear contract: one language per Makefile generation
**Alternatives Considered:** Auto-detection with priority order, detecting all languages and prompting user


## Decision 15: Security Scanning After Command Completion
**Date:** 2025-01-XX
**Decision:** Buffer command output and scan after completion, not during streaming
**Rationale:**
- `security.AnalyseContent()` requires complete content as string parameter
- Cannot process streaming data incrementally
- Maintains security protection while allowing real-time display
**Implementation:** Stream output to user in real-time, buffer copy for post-execution scanning

## Decision 16: Makefile Target Discovery at Initialization
**Date:** 2025-01-XX
**Decision:** Scan Makefile and report targets as capabilities when tool initializes
**Rationale:**
- Provides clear interface of available actions
- Auto-generates Makefile if missing before reporting
- No dynamic reloading - user's problem if they modify Makefile after start
**Alternatives Considered:** Dynamic reloading (rejected - adds complexity)

## Decision 17: No Git Repository Validation
**Date:** 2025-01-XX
**Decision:** Do not check if directory is a git repository before operations
**Rationale:**
- Simpler implementation
- Git commands provide clear error messages
- User can resolve by running `git init` or changing directory
**Implementation:** Pass git errors directly to user with helpful hint

## Decision 18: Node.js Uses npm run for All Commands
**Date:** 2025-01-XX
**Decision:** All Node.js targets use `npm run` prefix
**Rationale:**
- Consistent interface across all targets
- Leverages project's package.json scripts
- No ambiguity about which command to use
**Commands:** `npm run lint:fix`, `npm run format`, `npm run lint`, `npm test`

## Decision 19: Batch Git Add Operations
**Date:** 2025-01-XX
**Decision:** Execute `git add` as single batch operation for all files
**Rationale:**
- More efficient than individual operations
- Standard git practice
- Reduces command execution overhead
**Implementation:** `git add file1 file2 file3` instead of multiple `git add` calls

## Decision 20: Writability Check via Owner and Permissions
**Date:** 2025-01-XX
**Decision:** Verify writability by checking directory owner matches calling user and owner write bit is set
**Rationale:**
- Avoids creating temporary files for testing
- Direct permission check is faster
- Sufficient for security validation
**Alternatives Considered:** Create/delete temp file (rejected - side effects)


## Decision 21: No Security Scanning of Command Output
**Date:** 2025-01-XX
**Decision:** Do not scan command output for security threats
**Rationale:**
- `security.AnalyseContent()` requires complete content as string and cannot process streaming data
- Real-time output streaming is critical for user experience with long-running commands
- Buffering entire output would delay feedback and defeat the purpose of streaming
- Tool runs as the user in their own repository - no security boundary crossing
- Command output is from trusted tools (make, git) executing user's own code
**Alternatives Considered:**
- Buffer and scan after completion (rejected - breaks real-time streaming requirement)
- Scan incrementally (rejected - not supported by security framework)


## Design Phase Decisions

## Decision 22: Initialization at Tool Registration
**Date:** 2025-01-XX
**Decision:** Perform working directory validation, tool checking, and Makefile parsing during tool registration (init)
**Rationale:**
- Capabilities known immediately when tool is available
- Fail-fast if environment is invalid
- No per-request overhead for validation
- Consistent with other tools in codebase
**Implementation:** `init()` function calls `initialize()` method before registration

## Decision 23: Single Struct for All Operations
**Date:** 2025-01-XX
**Decision:** Use single `ProjectActionsTool` struct with operation parameter instead of separate tools
**Rationale:**
- Shared state (working directory, targets, tool availability)
- Simpler registration and management
- Consistent interface for all project actions
- Follows pattern of filesystem tool
**Operations:** make, add, commit, list (capabilities)

## Decision 24: Streaming via exec.Cmd Pipes
**Date:** 2025-01-XX
**Decision:** Use `exec.Cmd` stdout/stderr pipes for real-time output streaming
**Rationale:**
- Standard Go pattern for command execution
- Natural streaming without buffering
- Separate stdout/stderr handling
- Context cancellation support
**Implementation:** Set up pipes, start command, stream to result

## Decision 25: Makefile Templates as Constants
**Date:** 2025-01-XX
**Decision:** Store Makefile templates as Go string constants in map
**Rationale:**
- Simple, no external files needed
- Easy to maintain and version
- Fast access, no I/O
- Type-safe language selection
**Structure:** `map[string]string` with language keys

## Decision 26: No Makefile Reloading
**Date:** 2025-01-XX
**Decision:** Parse Makefile once at initialization, never reload
**Rationale:**
- Simpler implementation
- Predictable behavior
- User can restart tool if Makefile changes
- Consistent with requirements
**Trade-off:** User must restart tool after Makefile modifications

## Decision 27: Batch Git Add Implementation
**Date:** 2025-01-XX
**Decision:** Execute single `git add file1 file2 file3` command for multiple files
**Rationale:**
- More efficient than multiple commands
- Standard git practice
- Atomic operation
- Simpler error handling
**Implementation:** Build single command with all validated paths

## Decision 28: Commit Message via Stdin
**Date:** 2025-01-XX
**Decision:** Pass commit message to `git commit --file=-` via stdin
**Rationale:**
- Prevents shell interpretation
- No escaping needed
- Handles multi-line messages naturally
- Security best practice
**Implementation:** Use `cmd.Stdin = strings.NewReader(message)`

## Decision 29: Simple Error Types
**Date:** 2025-01-XX
**Decision:** Use custom error type with enum for error categories
**Rationale:**
- Clear error classification
- Consistent error messages
- Easy to test error paths
- Supports error wrapping
**Types:** InvalidDirectory, ToolNotFound, InvalidTarget, InvalidPath, CommandFailed, MakefileInvalid, CommitTooLarge

## Decision 30: No Command Retry Logic
**Date:** 2025-01-XX
**Decision:** Execute commands once, return result immediately
**Rationale:**
- Thin wrapper philosophy
- User/agent decides on retry
- Simpler implementation
- Predictable behavior
**Implementation:** Single `cmd.Run()` call, return result

## Decision 31: Capabilities as Separate Operation
**Date:** 2025-01-XX
**Decision:** Provide "list" operation that returns capabilities
**Rationale:**
- Explicit discovery mechanism
- Can be called anytime
- Returns current state
- Useful for debugging
**Response:** Working directory, make targets, tool availability, Makefile existence


## Decision 32: Fail Initialization if Tools Missing
**Date:** 2025-01-XX
**Decision:** Tool initialization must fail if make or git are not on PATH
**Rationale:**
- Tool cannot function without required commands
- Fail-fast principle - detect issues immediately
- Simpler struct without availability flags
- Clear error message to user at startup
**Implementation:** Check PATH during `initialize()`, return error if not found, tool won't register


## Decision 33: Separate Stdout and Stderr in Results
**Date:** 2025-01-XX
**Decision:** Stream and return stdout and stderr as separate fields in CommandResult
**Rationale:**
- Cannot combine stdout/stderr deterministically (separate streams, unpredictable interleaving)
- Real-time streaming required for long-running commands
- Allows caller to handle streams appropriately
- Preserves all output without loss
**Implementation:** Use separate pipes/buffers for stdout and stderr, stream both in real-time via goroutines, return both captured outputs in result


## Decision 34: No CapabilitiesResult
**Date:** 2025-01-XX
**Decision:** Remove CapabilitiesResult - tool capabilities declared in Definition() only
**Rationale:**
- Other tools (calculator, aws_documentation) don't report capabilities separately
- Tool schema in Definition() already declares available parameters
- Available operations are dynamic (Makefile .PHONY targets + git operations)
- Agent discovers operations by reading tool description and Makefile
- Simpler design without separate capabilities operation
**Alternatives Considered:** Separate capabilities operation like filesystem's list_allowed_directories (rejected - not needed for this tool)
