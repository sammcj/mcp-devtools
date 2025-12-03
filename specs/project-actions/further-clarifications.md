Further Clarifications

1. Update 3.2 to specify that the language must be specified when the tool is called. The hint text to the LLM should indicate that the LLM can use soft heuristics to determine the language, but only one language is allowed.
2. Node.js command ambiguity has been fixed.  `npm run` should be used in all cases.
3. No checking for git repositories should be attempted.  If the git commands fail due to being actioned in a non-git directory, they should be passed to the user to resolve, with the hint that they might need to run `git init` or specify another directory.
4. Analyse the security scanning capabilities of this code base to determine whether it can handle real time streaming.
5. When the tool is called, it should scan the Makefile for phony targets and report those as its capabilities, autogenerating the Makefile .  If the user adds targets after the tool has been started, they will not appear until the tool has been reloaded.  This is the user's problem.
6. If the Makefile has no .PHONY targets or is malformed, no actions will be returned to the model for use.  This is the user's problem.
7. Batch operation is acceptable for the add command.
8. PROJECT_ACTIONS_MAX_COMMIT_SIZE is good.

Minor clarifications

1.3 Writability should be verified by checking that the directory owner is the calling user, and that the owner write bit is set.
3.10 The instruction to call the tool again should be returned to the model as a diagnostic message from the tool call.
4.6 There's no need to specify the requirement twice.  If git is present when the tool is started, there's no need to validate it again when the add action is called.
6.5 Update the requirement to indicate that the output should be returned without modification, provided that the scan succeeds.
