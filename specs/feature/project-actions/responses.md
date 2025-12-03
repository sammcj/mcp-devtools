1. Integration

This tool should integrate with the existing security framework. We'll need to ensure that the dependency on the security framework is documented.  FILESYSTEM_TOOL_ALLOWED_DIRS is only relevant to the filesystem tool and is not relevant.  Read @docs/security.md and determine if there are any other requirements which we need to clarify.

2. Scope & Actions:

The tool should support tests, linters, and formatting only via integration with a project's Makefile.  It should read the Makefile, and permit actions which match the Makefile's meta-targets.  The meta targets should only support alphanumeric characters, underscore, and hyphen.  No other characters should be permitted, in order to minimise the amount of security filtering needed.  If the tool is called and the project does not have a Makefile, it should auto-detect the language in use in the current project and create a minimalist Makefile which supports the most common actions.  e.g. In a python project, it would create a Makefile containing the following:

    .PHONY: default
    default:
            echo "No default action"

    .PHONY: fix
    fix:
            ruff check --fix .

    .PHONY: format
    format:
            black .

    .PHONY: lint
    lint:
            ruff check .

    .PHONY: test
    test:
            python -m pytest

It must create the Makefile with tab separators rather than spaces as per Linux/Unix conventions.  If the language is unknown, the tool should return instructions for the LLM to generate a Makefile based on its own knowledge, or by reaching out using other tools.

The git operations which should be supported are add and commit.  The add functionality should support only relative filenames and nothing else.  The commit function should only support a single (possibly multi-line) commit message.  No other git commands will be implemented or supported.

3. Security & Safety:

A dynamic list of commands must be allowed, based on the state of the project's Makefile at the time of the tool's invocation.  Only commands marked as .PHONY should be allowed.  These tools will be invoked with `make TOOLNAME` and no other parameters.  The add and commit commands will be independently implemented directly via git.

The tool must only allow operation in the project directory or a subdirectory of it.

A dry-run mode should be supplied which shows the commands which would be run.

git commits should be performed with exactly the message passed by the model.  No validation or other formatting of the commit message should be performed.  For security purposes, the commit message will be provided to git on standard input via the `git commit --file=-` command, so that no shell interpretation or other dynamic command injection is possible.

4. Output & Feedback:

The tool must stream command output in real-time, from both standard output and standard error.  It should include exit codes and execution time in results for display by the model.

5. Configuration:

No per project configuration is needed other than what is provided in the Makefile.

6. Error Handling:

The tool should simply execute the required command (make or git), and return the output to the caller.  No attempt should be made to retry or handle errors.

make or git (depending on which is required) must be on the user's PATH when the tool is invoked.  If they are not it must emit a warning and exit.  No other attempts should be made to validate tooling.
