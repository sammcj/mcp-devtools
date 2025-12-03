Security Framework Clarifications

1. Command Execution Security:

- The tool should use the security helper functions if they are suitable for its operations.
- The tool should scan the command output for security threats using security.AnalyseContent() if it is compatible with streaming the results in real-time to the user.
- The Makefile content may be checked for security threats provided that this can be offloaded to the security module rather than manually implemented.

2. Working Directory Validation:

- Working Directory: Accept an optional working_directory parameter (defaults to os.Getwd())
    - working directory must not be / or any typical Linux or Mac OS system directories such as /bin, /lib, /usr/share/doc, etc.  It must be writable by the user running the tool, and should be displayed clearly when the tool is called.
- Path Validation: All operations must occur within the working directory or its subdirectories
- Makefile Location: Look for Makefile in the working directory
- Git Operations: Execute in the working directory
- Relative Paths: For add, validate that relative paths don't escape the working directory using filepath.Clean() and prefix checking

3. Makefile Parsing Security:

- The Makefile is provided by the user for their project so we shouldn't be heavy handed about what we deny or permit.  The Makefile is part of the code, and if it breaks the code, that's the user's problem.
- There is no need to scan commands in .PHONY targets.  This tool is not crossing any security boundaries (it runs as the user who owns the repository), and all it needs to do is validate that the target name contains only alphanumeric, hyphen, and underscore, so that no command injection trickery can take place.
- Use security helper functions for Makefile reading: ops.SafeFileRead("Makefile")

4. Git Operations Security:

- For add, all paths must resolve to files within the working directory.  It's fine for them to contain ../ but they should be turned into absolute paths and ensure that they lie within the intended target.
- The commit messages should be limited, but generous.  Start with 16 KB and allow it to be tuned via environment variables.
- Commit messages should not be scanned for sensitive data - that is the role of git hooks and out of scope.

5. Auto-generated Makefile Security:

- When auto-generating a Makefile, we control the content, so there's no need to scan it.  We should create it free of special characters, shell pipes, variables, etc.
- Basic Makefiles should be created automatically.
