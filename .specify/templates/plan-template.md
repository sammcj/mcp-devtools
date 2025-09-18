
# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

## Execution Flow (/plan command scope)
```
1. Load feature spec from Input path
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → Detect Project Type from context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Fill the Constitution Check section based on the content of the constitution document.
4. Evaluate Constitution Check section below
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
5. Execute Phase 0 → research.md
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
6. Execute Phase 1 → contracts, data-model.md, quickstart.md, agent-specific template file (e.g., `CLAUDE.md` for Claude Code, `.github/copilot-instructions.md` for GitHub Copilot, `GEMINI.md` for Gemini CLI, `QWEN.md` for Qwen Code or `AGENTS.md` for opencode).
7. Re-evaluate Constitution Check section
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
8. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md)
9. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:
- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary
[Extract from feature spec: primary requirement + technical approach from research]

## Technical Context
**Language/Version**: [e.g., Golang 1.13, Python 3.13 or NEEDS CLARIFICATION]
**Primary Dependencies**: [e.g., mcp-go, N/A or NEEDS CLARIFICATION]
**Storage**: [if applicable, e.g., PostgreSQL, JSON, in-memory or N/A]
**Testing**: [e.g., make test]
**Target Platform**: [Linux, macOS]
**Project Type**: [multi-tool MCP server]
**Performance Goals**: [domain-specific, e.g., 1000 req/s, completes in <2s, N/A or NEEDS CLARIFICATION]
**Constraints**: [domain-specific, e.g., <100MB memory, no files longer than 700 lines, offline-capable, tests must not rely on external services, NA or NEEDS CLARIFICATION]
**Scale/Scope**: [if applicable, domain-specific, e.g., 1M LOC, 50 screens, NA or NEEDS CLARIFICATION]

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

[Gates determined based on constitution file]
## Project Structure

### Core Documentation
```
docs/
├── creating-new-tools.md     # Tool Development Constitution
├── security.md               # Security system documentation
├── oauth/                    # OAuth integration guides
└── tools/                    # Individual tool documentation
    └── overview.md           # Tool registry and capabilities
    └── [tool name].md        # Per-tool documentation
```

### Source Code
```
internal/
├── tools/                # Tool implementations
│   ├── [category]/       # Tool categories (api, filesystem, etc.)
│   │   └── [tool-name]/  # Individual tool packages
│   │       └── tool.go   # Tool implementation
│   ├── tools.go          # Tool interface definition
│   └── enablement.go     # Tool enablement management
│
├── registry/             # Tool registration system
│   └── registry.go       # Central registry with requiresEnablement()
│
├── imports/              # Import management
│   └── tools.go          # Tool import registry (NOT main.go)
│
├── security/             # Security framework
│   ├── manager.go        # Security manager
│   ├── helpers.go        # SafeHTTPGet, SafeFileRead helpers
│   ├── analyser.go       # Content analysis
│   └── patterns.go       # Security patterns and rules
│
├── oauth/                # OAuth client implementation
├── handlers/             # Package-specific handlers
└── config/               # Global configuration
tests/
├── tools/                # Tool-specific tests
├── unit/                 # Unit tests
├── security/             # Security middleware tests
└── testutils/            # Test utilities
```

### Build Artifacts
```
bin/
└── mcp-devtools         # Compiled MCP server binary

mcp.json                 # Example MCP client config
Makefile                 # Build, lint and test automation
```

**Structure Decision**: Tools are organised by functional category with strict interface conformity. Each tool is self-contained in its own package under `internal/tools/[category]/[tool-name]/`. The registry system manages discovery and enablement, whilst the security framework provides mandatory integration points for file and network operations.

## Phase 0: Outline & Research
1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:
   ```
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md with all NEEDS CLARIFICATION resolved

## Phase 1: Design & Contracts
*Prerequisites: research.md complete*

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

3. **Generate contract tests** from contracts:
   - One test file per endpoint
   - Assert request/response schemas
   - Tests must fail (no implementation yet)

4. **Extract test scenarios** from user stories:
   - Each story → integration test scenario
   - Quickstart test = story validation steps

5. **Update agent file incrementally** (O(1) operation):
   - Run `.specify/scripts/bash/update-agent-context.sh claude` for your AI assistant
   - If exists: Add only NEW tech from current plan
   - Preserve manual additions between markers
   - Update recent changes (keep last 3)
   - Keep under 150 lines for token efficiency
   - Output to repository root

**Output**: data-model.md, /contracts/*, failing tests, quickstart.md, agent-specific file

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:
- Load `.specify/templates/tasks-template.md` as base
- Generate tasks from Phase 1 design docs (contracts, data model, quickstart)
- Each contract → contract test task [P]
- Each entity → model creation task [P]
- Each user story → integration test task
- Implementation tasks to make tests pass

**Ordering Strategy**:
- TDD order: Tests before implementation
- Dependency order: Models before services before UI
- Mark [P] for parallel execution (independent files)

**Estimated Output**: 25-30 numbered, ordered tasks in tasks.md

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)
**Phase 4**: Implementation (execute tasks.md following constitutional principles)
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [ ] Phase 0: Research complete (/plan command)
- [ ] Phase 1: Design complete (/plan command)
- [ ] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [ ] Initial Constitution Check: PASS
- [ ] Post-Design Constitution Check: PASS
- [ ] All NEEDS CLARIFICATION resolved
- [ ] Complexity deviations documented

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*
