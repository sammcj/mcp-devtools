---
references:
    - specs/project-actions/requirements.md
    - specs/project-actions/design.md
---
# Project Actions Tool Implementation

- [x] 1. Create tool struct and basic registration
  - [x] 1.1. Create internal/tools/projectactions/project_actions.go with ProjectActionsTool struct
  - [x] 1.2. Implement init() function with tool registration

- [x] 2. Implement working directory validation
  - [x] 2.1. Create validateWorkingDirectory() method
  - [x] 2.2. Add system directory blocking logic
  - [x] 2.3. Add writability check via owner and permissions

- [x] 3. Implement tool availability checking
  - [x] 3.1. Create checkToolAvailability() helper function
  - [x] 3.2. Check for make and git on PATH during initialization
  - [x] 3.3. Return error if required tools not found

- [x] 4. Create error types and constants
  - [x] 4.1. Define ProjectActionsError struct with ErrorType enum
  - [x] 4.2. Add error message constants

- [x] 5. Implement Makefile reading with security integration
  - [x] 5.1. Use security.Operations.SafeFileRead() for Makefile
  - [x] 5.2. Handle SecurityError responses appropriately

- [x] 6. Implement .PHONY target parsing
  - [x] 6.1. Create parsePhonyTargets() method
  - [x] 6.2. Extract .PHONY target names using regex
  - [x] 6.3. Validate target names (alphanumeric, hyphen, underscore only)

- [x] 7. Create Makefile templates for all languages
  - [x] 7.1. Define makefileTemplates map with language keys
  - [x] 7.2. Create Python template with ruff, black, pytest targets
  - [x] 7.3. Create Rust template with cargo targets
  - [x] 7.4. Create Go template with go fmt, golangci-lint targets
  - [x] 7.5. Create Node.js template with npm run targets
  - [x] 7.6. Ensure all templates use tabs for indentation

- [x] 8. Implement Makefile generation
  - [x] 8.1. Create generateMakefile() method
  - [x] 8.2. Validate language parameter
  - [x] 8.3. Return diagnostic message for invalid language
  - [x] 8.4. Write generated Makefile to working directory

- [x] 9. Implement make target execution
  - [x] 9.1. Create executeMakeTarget() method
  - [x] 9.2. Validate target exists in makefileTargets list
  - [x] 9.3. Build command: make TARGET_NAME
  - [x] 9.4. Execute with streaming output
  - [x] 9.5. Return CommandResult with exit code and timing

- [ ] 10. Implement real-time output streaming
  - [ ] 10.1. Create executeCommand() helper method
  - [ ] 10.2. Set up separate stdout/stderr pipes
  - [ ] 10.3. Stream both outputs in real-time via goroutines
  - [ ] 10.4. Capture exit code and execution duration
  - [ ] 10.5. Return CommandResult with all captured data

- [ ] 11. Add dry-run mode support
  - [ ] 11.1. Check dry_run parameter in all operations
  - [ ] 11.2. Display exact commands without execution when dry_run=true
  - [ ] 11.3. Validate all parameters in dry-run mode

- [ ] 12. Implement path validation and resolution
  - [ ] 12.1. Create validateAndResolvePath() method
  - [ ] 12.2. Use filepath.Clean() and filepath.Abs()
  - [ ] 12.3. Verify resolved paths are within working directory

- [ ] 13. Implement git add (batch)
  - [ ] 13.1. Create executeGitAdd() method
  - [ ] 13.2. Accept array of relative file paths
  - [ ] 13.3. Validate and resolve each path
  - [ ] 13.4. Build single git add command with all paths
  - [ ] 13.5. Execute with streaming output
  - [ ] 13.6. Include hint about git init in error messages

- [ ] 14. Implement git commit with stdin
  - [ ] 14.1. Create executeGitCommit() method
  - [ ] 14.2. Accept single commit message parameter
  - [ ] 14.3. Validate message size against limit
  - [ ] 14.4. Support PROJECT_ACTIONS_MAX_COMMIT_SIZE env var
  - [ ] 14.5. Pass message via stdin using git commit --file=-
  - [ ] 14.6. Execute in working directory with streaming output
  - [ ] 14.7. Include hint about git init in error messages

- [ ] 15. Implement MCP tool Definition()
  - [ ] 15.1. Create Definition() method returning mcp.Tool
  - [ ] 15.2. Add tool description with operation details
  - [ ] 15.3. Define operation parameter (make targets + add/commit)
  - [ ] 15.4. Add MCP destructive annotation

- [ ] 16. Implement Execute() method
  - [ ] 16.1. Create Execute() method with MCP signature
  - [ ] 16.2. Parse operation parameter
  - [ ] 16.3. Route to appropriate handler (make/add/commit)
  - [ ] 16.4. Display working directory in output

- [ ] 17. Create types.go with data models
  - [ ] 17.1. Define CommandResult struct
  - [ ] 17.2. Define ToolArgs struct for input parameters

- [ ] 18. Write unit tests for working directory validation
  - [ ] 18.1. Test valid directory acceptance
  - [ ] 18.2. Test system directory rejection
  - [ ] 18.3. Test writability check

- [ ] 19. Write unit tests for Makefile parsing
  - [ ] 19.1. Test valid target name extraction
  - [ ] 19.2. Test invalid target name rejection
  - [ ] 19.3. Test malformed Makefile handling

- [ ] 20. Write unit tests for path validation
  - [ ] 20.1. Test path resolution with filepath.Clean/Abs
  - [ ] 20.2. Test path traversal prevention
  - [ ] 20.3. Test working directory containment

- [ ] 21. Write unit tests for Makefile generation
  - [ ] 21.1. Test Python template generation
  - [ ] 21.2. Test Rust template generation
  - [ ] 21.3. Test Go template generation
  - [ ] 21.4. Test Node.js template generation
  - [ ] 21.5. Verify tab indentation in all templates

- [ ] 22. Write integration tests for make target execution
  - [ ] 22.1. Create temp directory with test Makefile
  - [ ] 22.2. Execute make target and verify output
  - [ ] 22.3. Verify exit code and timing capture

- [ ] 23. Write integration tests for git operations
  - [ ] 23.1. Initialize test git repository
  - [ ] 23.2. Test git add with multiple files
  - [ ] 23.3. Test git commit with stdin message
  - [ ] 23.4. Verify git state after operations

- [ ] 24. Write tests for dry-run mode
  - [ ] 24.1. Test dry-run displays commands without execution
  - [ ] 24.2. Test dry-run validates all parameters

- [ ] 25. Write tests for error handling
  - [ ] 25.1. Test command failure propagation
  - [ ] 25.2. Test no automatic retry on failure
  - [ ] 25.3. Test immediate error return

- [ ] 26. Write tests for security integration
  - [ ] 26.1. Test SafeFileRead usage for Makefile
  - [ ] 26.2. Test SecurityError handling
  - [ ] 26.3. Test security warning logging

- [ ] 27. Add comprehensive error messages
  - [ ] 27.1. Add all error message constants
  - [ ] 27.2. Include git init hint in git error messages

- [ ] 28. Implement extended help via ProvideExtendedInfo()
  - [ ] 28.1. Create ProvideExtendedInfo() method
  - [ ] 28.2. Add usage examples for all operations
  - [ ] 28.3. Add troubleshooting tips

- [ ] 29. Write tool documentation
  - [ ] 29.1. Create docs/tools/project-actions.md
  - [ ] 29.2. Document security framework dependency
  - [ ] 29.3. Document all operations and parameters
  - [ ] 29.4. Add examples for each language template

- [ ] 30. Update main README.md
  - [ ] 30.1. Add project-actions to tools table
  - [ ] 30.2. Document ENABLE_ADDITIONAL_TOOLS requirement
