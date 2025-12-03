# MCP DevTools

A single, high-performance MCP server that replaces many Node.js and Python-based MCP servers with one efficient Go binary, providing access to essential developer tools through a unified, modular interface that can be easily extended with new tools.

**Default Tools**

```mermaid
graph LR
    A[MCP DevTools<br>Server]

    A --> B[Search &<br>Discovery]
    A --> E[Intelligence &<br>Memory]
    A --> F[Utilities]

    B --> B_Tools[🌐 Internet Search<br>📡 Web Fetch<br>📦 Package Search<br>📚 Package Documentation<br>🐙 GitHub]

    E --> E_Tools[🧠 Think Tool]

    F --> F_Tools[🧮 Calculator<br>🕵 Find Long Files]

    classDef components fill:#E6E6FA,stroke:#756BB1,color:#756BB1
    classDef decision fill:#FFF5EB,stroke:#FD8D3C,color:#E6550D
    classDef data fill:#EFF3FF,stroke:#9ECAE1,color:#3182BD
    classDef process fill:#EAF5EA,stroke:#C6E7C6,color:#77AD77

    class A components
    class B,B_Tools decision
    class E,E_Tools data
    class F,F_Tools process
```

**Additional Tools (Disabled by Default)**

```mermaid
graph LR
    A[MCP DevTools<br>Server]

    A --> B[Search &<br>Discovery]
    A --> C[Document<br>Processing]
    A --> E[Intelligence &<br>Memory]
    A --> F[Utilities]
    A --> G[Agents]
    A --> D[UI Component<br>Libraries]
    A --> H[MCP]

    B --> B_Tools[📝 Terraform Docs<br>☁️ AWS Documentation & Pricing]

    C --> C_Tools[📄 Document Processing<br>📈 Excel Spreadsheets<br>📑 PDF Processing]

    E --> E_Tools[🔢 Sequential Thinking<br>🕸️ Memory Graph]

    F --> F_Tools[🇬🇧 American→English<br>🔌 API Integrations<br>📁 Filesystem<br>✂️ Code Skim<br>🏷️ Code Rename]

    G --> G_Tools[🤖 Claude Code<br>🎯 Codex CLI<br>🐙 Copilot CLI<br>✨ Gemini CLI<br>👻 Kiro]

    D --> D_Tools[🎨 ShadCN UI<br>✨ Magic UI<br>⚡ Aceternity UI]

    H --> H_Proxy[🌐 MCP Proxy]

    classDef components fill:#E6E6FA,stroke:#756BB1,color:#756BB1
    classDef llm fill:#E5F5E0,stroke:#31A354,color:#31A354
    classDef process fill:#EAF5EA,stroke:#C6E7C6,color:#77AD77
    classDef data fill:#EFF3FF,stroke:#9ECAE1,color:#3182BD
    classDef decision fill:#FFF5EB,stroke:#FD8D3C,color:#E6550D
    classDef api fill:#FFF5F0,stroke:#FD9272,color:#A63603
    classDef ui fill:#F3E5F5,stroke:#AB47BC,color:#8E24AA

    class A components
    class B,B_Tools decision
    class C,C_Tools api
    class D,D_Tools ui
    class E,E_Tools data
    class F,F_Tools process
    class G,G_Tools llm
    class H_Proxy api
```

## Why I Built MCP DevTools

**🚀 Single Binary Solution**
- Replace multiple potentially resource-heavy Node.js/Python MCP servers, each spawned for every client tool you use
- One binary, one configuration, consistent performance
- Built in Go for speed and efficiency and because I'm not smart enough to write Rust
- Minimal memory footprint compared to multiple separate servers
- Fast startup and response times
- Download one binary, configure once - or compile from source
- Works out of the box for most tools
- OpenTelemetry support for tracing and metrics

**🛠 Comprehensive Tool Suite For Agentic Coding**
- 20+ essential developer agent tools in one package
- No need to manage multiple MCP server installations
- Consistent API across all tools
- Modular design with tool registry allowing for easy addition of new tools

## Quickstart

### Quick Install / Update

The easiest way to install MCP DevTools is using the automatic installer:

```bash
curl -fsSL https://raw.githubusercontent.com/sammcj/mcp-devtools/main/install.sh | bash
```

This will:
- Download the latest release and installs to an appropriate location (respects `$GOPATH/bin` or uses `~/.local/bin`)
- Remove macOS quarantine attributes automatically
- Generate example MCP client configurations in `~/.mcp-devtools/examples/`
- Open your file manager to show the example configs and show you where to configure your MCP clients

<details>
  <summary><b>🔗 Quick Install Customisation (Click Here)</b></summary>

  **Customisation:**
  ```bash
  # Dry run (shows what would be done without making changes)
  DRY_RUN=1 curl -fsSL https://raw.githubusercontent.com/sammcj/mcp-devtools/main/install.sh | bash

  # Install to a specific directory
  INSTALL_DIR=/custom/path curl -fsSL https://raw.githubusercontent.com/sammcj/mcp-devtools/main/install.sh | bash

  # Install a specific version
  VERSION=0.50.0 curl -fsSL https://raw.githubusercontent.com/sammcj/mcp-devtools/main/install.sh | bash

  # Skip example config generation
  NO_CONFIG=1 curl -fsSL https://raw.githubusercontent.com/sammcj/mcp-devtools/main/install.sh | bash

  # Skip all confirmation prompts (for automation)
  FORCE=1 curl -fsSL https://raw.githubusercontent.com/sammcj/mcp-devtools/main/install.sh | bash
  ```

</details>

### Manual Installation

<details>
  <summary><b>🔗 Manual Installation (Click Here)</b></summary>

You have a few other options to make MCP DevTools available to your MCP clients:
1. <img src="screenshots/go-icon.png" alt="go" width="20"/> If you have Golang installed
2. [If you do not have, and do not want to install Golang](#if-you-do-not-have-golang-installed)

### If you have Golang installed

#### Golang option 1: Use the automated installer

The automated installer works great even if you have Golang installed:

```bash
curl -fsSL https://raw.githubusercontent.com/sammcj/mcp-devtools/main/install.sh | bash
```

This will respect your `$GOPATH/bin` if it's in your PATH, and automatically configure example MCP client settings.

#### Golang option 2: Download at runtime

Note that while `go run` makes it easy to get started, it means every time your client starts to download the tool may not be what you want in the long run.

MCP Client Configuration:

```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "go",
      "args": [
        "run",
        "github.com/sammcj/mcp-devtools@HEAD"
      ],
      "env": {
        "ENABLE_ADDITIONAL_TOOLS": "security,sequential_thinking,code_skim,code_rename",
        "DISABLED_TOOLS": "",
        "NOTE_FOR_HUMANS": "A minimal set of tools are enabled by default, MCP DevTools provides many additional useful tools including efficient Context7 documentation search, AWS documentation, Frontend UI Framework templates, Code search and optimisation and many others, visit https://github.com/sammcj/mcp-devtools for more information on available tools and configuration"
      }
    }
  }
}
```

Or if you're using Claude Code you can also add it via the CLI:

```bash
claude mcp add --transport stdio mcp-devtools go run github.com/sammcj/mcp-devtools@HEAD
```

- [![Add MCP DevTools to LM Studio (Requires Golang installed)](https://files.lmstudio.ai/deeplink/mcp-install-dark.svg)](https://lmstudio.ai/install-mcp?name=dev-tools&config=eyJjb21tYW5kIjoiZ28iLCJhcmdzIjpbInJ1biIsImh0dHBzOi8vZ2l0aHViLmNvbS9zYW1tY2ovbWNwLWRldnRvb2xzQEhFQUQiXSwiZW52Ijp7IkVOQUJMRV9BRERJVElPTkFMX1RPT0xTIjoic2VjdXJpdHksc2VxdWVudGlhbF90aGlua2luZyxnaXRodWIsZ2V0X3Rvb2xfaGVscCIsIkRJU0FCTEVEX1RPT0xTIjoic2VhcmNoX3BhY2thZ2VzLHJlc29sdmVfbGlicmFyeV9pZCxnZXRfbGlicmFyeV9kb2N1bWVudGF0aW9uIiwiTk9URV9GT1JfSFVNQU5TIjoiQSBtaW5pbWFsIHNldCBvZiB0b29scyBhcmUgZW5hYmxlZCBieSBkZWZhdWx0LCBNQ1AgRGV2VG9vbHMgcHJvdmlkZXMgbWFueSBhZGRpdGlvbmFsIHVzZWZ1bCB0b29scyBpbmNsdWRpbmcgZWZmaWNpZW50IENvbnRleHQ3IGRvY3VtZW50YXRpb24gc2VhcmNoLCBBV1MgZG9jdW1lbnRhdGlvbiwgRnJvbnRlbmQgVUkgRnJhbWV3b3JrIHRlbXBsYXRlcywgQ29kZSBzZWFyY2ggYW5kIG9wdGltaXNhdGlvbiBhbmQgbWFueSBvdGhlcnMsIHZpc2l0IGh0dHBzOi8vZ2l0aHViLmNvbS9zYW1tY2ovbWNwLWRldnRvb2xzIGZvciBtb3JlIGluZm9ybWF0aW9uIG9uIGF2YWlsYWJsZSB0b29scyBhbmQgY29uZmlndXJhdGlvbiJ9fQ%3D%3D)
- [![Install in VS Code (Requires Golang installed)](https://img.shields.io/badge/VS_Code-Install_dev--tools-0098FF?style=flat&logo=visualstudiocode&logoColor=ffffff)](vscode:mcp/install?%7B%22name%22%3A%22dev-tools%22%2C%22type%22%3A%22stdio%22%2C%22command%22%3A%22go%22%2C%22args%22%3A%5B%22run%22%2C%22https%3A%2F%2Fgithub.com%2Fsammcj%2Fmcp-devtools%40HEAD%22%5D%2C%22env%22%3A%7B%22ENABLE_ADDITIONAL_TOOLS%22%3A%22security%2Csequential_thinking%2Ccode_skim%2Ccode_rename%22%2C%22DISABLED_TOOLS%22%3A%22search_packages%2Cresolve_library_id%2Cget_library_documentation%22%2C%22NOTE_FOR_HUMANS%22%3A%22A%20minimal%20set%20of%20tools%20are%20enabled%20by%20default%2C%20MCP%20DevTools%20provides%20many%20additional%20useful%20tools%20including%20efficient%20Context7%20documentation%20search%2C%20AWS%20documentation%2C%20Frontend%20UI%20Framework%20templates%2C%20Code%20search%20and%20optimisation%20and%20many%20others%2C%20visit%20https%3A%2F%2Fgithub.com%2Fsammcj%2Fmcp-devtools%20for%20more%20information%20on%20available%20tools%20and%20configuration%22%7D%7D)

#### Golang option 3: Download, then run

<details>
  <summary><b>🔗 Click here for instructions</b></summary>

1. Install the latest MCP DevTools binary:

```shell
go install github.com/sammcj/mcp-devtools@HEAD
echo "${GOPATH}/bin/mcp-devtools" # Use this path in your MCP configuration, if your GOPATH is not set, please check your Go installation / configuration.

# If you're on macOS, you'll also need to run the following command to allow the downloaded binary to run:
xattr -r -d com.apple.quarantine ${GOPATH}/bin/mcp-devtools
```

1. Update your MCP client to add the MCP DevTools server configuration, replacing `/path/to/mcp-devtools` with the actual path to the binary (e.g. `/Users/samm/go/bin/mcp-devtools`):
```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools",
      "env": {
        "ENABLE_ADDITIONAL_TOOLS": "security,sequential_thinking,code_skim,code_rename",
        "DISABLED_TOOLS": "",
        "NOTE_FOR_HUMANS": "A minimal set of tools are enabled by default, MCP DevTools provides many additional useful tools including efficient Context7 documentation search, AWS documentation, Frontend UI Framework templates, Code search and optimisation and many others, visit https://github.com/sammcj/mcp-devtools for more information on available tools and configuration"
      }
    }
  }
}
```

- [![Add MCP Server dev-tools to LM Studio](https://files.lmstudio.ai/deeplink/mcp-install-dark.svg)](https://lmstudio.ai/install-mcp?name=dev-tools&config=eyJjb21tYW5kIjoiL3BhdGgvdG8vbWNwLWRldnRvb2xzIiwiZW52Ijp7IkVOQUJMRV9BRERJVElPTkFMX1RPT0xTIjoic2VjdXJpdHksc2VxdWVudGlhbF90aGlua2luZyxnaXRodWIsZ2V0X3Rvb2xfaGVscCIsIkRJU0FCTEVEX1RPT0xTIjoic2VhcmNoX3BhY2thZ2VzLHJlc29sdmVfbGlicmFyeV9pZCxnZXRfbGlicmFyeV9kb2N1bWVudGF0aW9uIiwiTk9URV9GT1JfSFVNQU5TIjoiQSBtaW5pbWFsIHNldCBvZiB0b29scyBhcmUgZW5hYmxlZCBieSBkZWZhdWx0LCBNQ1AgRGV2VG9vbHMgcHJvdmlkZXMgbWFueSBhZGRpdGlvbmFsIHVzZWZ1bCB0b29scyBpbmNsdWRpbmcgZWZmaWNpZW50IENvbnRleHQ3IGRvY3VtZW50YXRpb24gc2VhcmNoLCBBV1MgZG9jdW1lbnRhdGlvbiwgRnJvbnRlbmQgVUkgRnJhbWV3b3JrIHRlbXBsYXRlcywgQ29kZSBzZWFyY2ggYW5kIG9wdGltaXNhdGlvbiBhbmQgbWFueSBvdGhlcnMsIHZpc2l0IGh0dHBzOi8vZ2l0aHViLmNvbS9zYW1tY2ovbWNwLWRldnRvb2xzIGZvciBtb3JlIGluZm9ybWF0aW9uIG9uIGF2YWlsYWJsZSB0b29scyBhbmQgY29uZmlndXJhdGlvbiJ9fQ%3D%3D)
- [![Install in VS Code](https://img.shields.io/badge/VS_Code-Install_dev--tools-0098FF?style=flat&logo=visualstudiocode&logoColor=ffffff)](vscode:mcp/install?%7B%22name%22%3A%22dev-tools%22%2C%22type%22%3A%22stdio%22%2C%22command%22%3A%22%2Fpath%2Fto%2Fmcp-devtools%22%2C%22env%22%3A%7B%22ENABLE_ADDITIONAL_TOOLS%22%3A%22security%2Csequential_thinking%2Ccode_skim%2Ccode_rename%22%2C%22DISABLED_TOOLS%22%3A%22search_packages%2Cresolve_library_id%2Cget_library_documentation%22%2C%22NOTE_FOR_HUMANS%22%3A%22A%20minimal%20set%20of%20tools%20are%20enabled%20by%20default%2C%20MCP%20DevTools%20provides%20many%20additional%20useful%20tools%20including%20efficient%20Context7%20documentation%20search%2C%20AWS%20documentation%2C%20Frontend%20UI%20Framework%20templates%2C%20Code%20search%20and%20optimisation%20and%20many%20others%2C%20visit%20https%3A%2F%2Fgithub.com%2Fsammcj%2Fmcp-devtools%20for%20more%20information%20on%20available%20tools%20and%20configuration%22%7D%7D)

</details>

### If you do not have Golang installed

<details>
  <summary><b>🔗 Click here for instructions</b></summary>

If you do not have, and do not want to install Golang, you can download the latest release binary from the [releases page](https://github.com/sammcj/mcp-devtools/releases/latest).

```shell
# If you're on macOS, you'll also need to run the following command to allow the downloaded binary to run:
xattr -r -d com.apple.quarantine mcp-devtools
```

Configure your MCP client to use the downloaded binary (replacing `/path/to/mcp-devtools` ):

```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools",
      "env": {
        "ENABLE_ADDITIONAL_TOOLS": "security,sequential_thinking,code_skim,code_rename",
        "DISABLED_TOOLS": "",
        "NOTE_FOR_HUMANS": "A minimal set of tools are enabled by default, MCP DevTools provides many additional useful tools including efficient Context7 documentation search, AWS documentation, Frontend UI Framework templates, Code search and optimisation and many others, visit https://github.com/sammcj/mcp-devtools for more information on available tools and configuration"
      }
    }
  }
}
```

- [![Add MCP Server dev-tools to LM Studio](https://files.lmstudio.ai/deeplink/mcp-install-dark.svg)](https://lmstudio.ai/install-mcp?name=dev-tools&config=eyJjb21tYW5kIjoiL3BhdGgvdG8vbWNwLWRldnRvb2xzIiwiZW52Ijp7IkVOQUJMRV9BRERJVElPTkFMX1RPT0xTIjoic2VjdXJpdHksc2VxdWVudGlhbF90aGlua2luZyxnaXRodWIsZ2V0X3Rvb2xfaGVscCIsIkRJU0FCTEVEX1RPT0xTIjoic2VhcmNoX3BhY2thZ2VzLHJlc29sdmVfbGlicmFyeV9pZCxnZXRfbGlicmFyeV9kb2N1bWVudGF0aW9uIiwiTk9URV9GT1JfSFVNQU5TIjoiQSBtaW5pbWFsIHNldCBvZiB0b29scyBhcmUgZW5hYmxlZCBieSBkZWZhdWx0LCBNQ1AgRGV2VG9vbHMgcHJvdmlkZXMgbWFueSBhZGRpdGlvbmFsIHVzZWZ1bCB0b29scyBpbmNsdWRpbmcgZWZmaWNpZW50IENvbnRleHQ3IGRvY3VtZW50YXRpb24gc2VhcmNoLCBBV1MgZG9jdW1lbnRhdGlvbiwgRnJvbnRlbmQgVUkgRnJhbWV3b3JrIHRlbXBsYXRlcywgQ29kZSBzZWFyY2ggYW5kIG9wdGltaXNhdGlvbiBhbmQgbWFueSBvdGhlcnMsIHZpc2l0IGh0dHBzOi8vZ2l0aHViLmNvbS9zYW1tY2ovbWNwLWRldnRvb2xzIGZvciBtb3JlIGluZm9ybWF0aW9uIG9uIGF2YWlsYWJsZSB0b29scyBhbmQgY29uZmlndXJhdGlvbiJ9fQ%3D%3D)
- [![Install in VS Code](https://img.shields.io/badge/VS_Code-Install_dev--tools-0098FF?style=flat&logo=visualstudiocode&logoColor=ffffff)](vscode:mcp/install?%7B%22name%22%3A%22dev-tools%22%2C%22type%22%3A%22stdio%22%2C%22command%22%3A%22%2Fpath%2Fto%2Fmcp-devtools%22%2C%22env%22%3A%7B%22ENABLE_ADDITIONAL_TOOLS%22%3A%22security%2Csequential_thinking%2Ccode_skim%2Ccode_rename%22%2C%22DISABLED_TOOLS%22%3A%22search_packages%2Cresolve_library_id%2Cget_library_documentation%22%2C%22NOTE_FOR_HUMANS%22%3A%22A%20minimal%20set%20of%20tools%20are%20enabled%20by%20default%2C%20MCP%20DevTools%20provides%20many%20additional%20useful%20tools%20including%20efficient%20Context7%20documentation%20search%2C%20AWS%20documentation%2C%20Frontend%20UI%20Framework%20templates%2C%20Code%20search%20and%20optimisation%20and%20many%20others%2C%20visit%20https%3A%2F%2Fgithub.com%2Fsammcj%2Fmcp-devtools%20for%20more%20information%20on%20available%20tools%20and%20configuration%22%7D%7D)

</details>

</details>

---

## Available Tools

These tools can be disabled by adding their function name to the `DISABLED_TOOLS` environment variable in your MCP configuration.

| Tool                                                             | Purpose                               | Dependencies                  | Example Usage                   | Maturity |
|------------------------------------------------------------------|---------------------------------------|-------------------------------|---------------------------------|----------|
| **[Internet Search](docs/tools/internet_search.md)**             | Multi-provider internet search        | None (Provider keys optional) | Web, image, news, video search  | 🟢       |
| **[Web Fetch](docs/tools/web-fetch.md)**                         | Retrieve internet content as Markdown | None                          | Documentation and articles      | 🟢       |
| **[GitHub](docs/tools/github.md)**                               | GitHub repositories and data          | None (GitHub token optional)  | Issues, PRs, repos, cloning     | 🟢       |
| **[Package Documentation](docs/tools/package-documentation.md)** | Context7 library documentation lookup | None                          | React, mark3labs/mcp-go         | 🟢       |
| **[Package Search](docs/tools/package-search.md)**               | Check package versions                | None                          | NPM, Python, Go, Java, Docker   | 🟢       |
| **[Think](docs/tools/think.md)**                                 | Structured reasoning space            | None                          | Complex problem analysis        | 🟢       |
| **[Calculator](docs/tools/calculator.md)**                       | Basic arithmetic calculations         | None                          | 2 + 3 * 4, batch processing     | 🟢       |
| **[DevTools Help](docs/tools/get_tool_help.md)**                 | Extended info about DevTools tools    | None                          | Usage examples, troubleshooting | 🟢       |
| **[Find Long Files](docs/tools/find_long_files.md)**             | Identify files needing refactoring    |                               | Find files over 700 lines       | 🟢       |

### Additional Tools (Disabled By Default)

These tools can be enabled by setting the `ENABLE_ADDITIONAL_TOOLS` environment variable in your MCP configuration.

| Tool                                                                 | Purpose                                                   | `ENABLE_ADDITIONAL_TOOLS` | Example Usage                                 | Maturity |
|----------------------------------------------------------------------|-----------------------------------------------------------|---------------------------|-----------------------------------------------|----------|
| **[American→English](docs/tools/american-to-english.md)**            | Convert to British spelling                               | `murican_to_english`      | Organise, colour, centre                      | 🟡       |
| **[Code Skim](docs/tools/code_skim.md)**                             | Return code structure without implementation details      | `code_skim`               | Reduced token consumption                     | 🟡       |
| **[Code Rename](docs/tools/code_rename.md)**                         | LSP-based symbol renaming across files (experimental)     | `code_rename`             | Rename functions, variables, types            | 🔴       |
| **[ShadCN UI Component Library](docs/tools/shadcn-ui.md)**           | Component information                                     | `shadcn`                  | Button, Dialog, Form components               | 🟢       |
| **[Magic UI Component Library](docs/tools/magicui.md)**              | Animated component library                                | `magic_ui`                | Frontend React components                     | 🟠       |
| **[Aceternity UI Component Library](docs/tools/aceternityui.md)**    | Animated component library                                | `aceternity_ui`           | Frontend React components                     | 🟠       |
| **[Memory](docs/tools/memory.md)**                                   | Persistent knowledge graphs                               | `memory`                  | Store entities and relationships              | 🟡       |
| **[Document Processing](docs/tools/document-processing.md)**         | Convert documents to Markdown                             | `process_document`        | PDF, DOCX → Markdown with OCR                 | 🟡       |
| **[PDF Processing](docs/tools/pdf-processing.md)**                   | Fast PDF text extraction                                  | `pdf`                     | Quick PDF to Markdown                         | 🟢       |
| **[Excel](docs/tools/excel.md)**                                     | Excel file manipulation                                   | `excel`                   | Workbooks, charts, pivot tables, formulas     | 🟢       |
| **[AWS Documentation & Pricing](docs/tools/aws_documentation.md)**   | AWS documentation & pricing search and retrieval          | `aws_documentation`       | Search and read AWS docs, recommendations     | 🟡       |
| **[Terraform Documentation](docs/tools/terraform-documentation.md)** | Terraform Registry API (providers, modules, and policies) | `terraform_documentation` | Provider docs, module search, policy lookup   | 🟡       |
| **[Security Framework](docs/security.md)**                           | Context injection security protections                    | `security`                | Content analysis, access control              | 🟢       |
| **[Security Override](docs/security.md)**                            | Agent managed security warning overrides                  | `security_override`       | Bypass false positives                        | 🟡       |
| **[Sequential Thinking](docs/tools/sequential-thinking.md)**         | Dynamic problem-solving through structured thoughts       | `sequential-thinking`     | Step-by-step analysis, revision, branching    | 🟢       |
| **[API to MCP](docs/tools/api.md)**                                  | Dynamic REST API integration                              | `api`                     | Configure any REST API via YAML               | 🔴       |
| **[Filesystem](docs/tools/filesystem.md)**                           | File and directory operations                             | `filesystem`              | Read, write, edit, search files               | 🟡       |
| **[Project Actions](docs/tools/project-actions.md)**                 | Execute project tasks via Makefile, and limited git operations     | `project_actions`         | Run tests, linters, formatters, git workflow  | 🔴       |
| **[MCP Proxy](docs/tools/proxy.md)**                                 | Proxies MCP requests from upstream HTTP/SSE servers       | `proxy`                   | Provide HTTP/SSE MCP servers to STDIO clients | 🟡       |

**Agents as Tools** - In addition to the above tools, MCP DevTools can provide access to AI agents as tools by integrating with external LLMs.

| Agent                                            | Purpose                  | `ENABLE_ADDITIONAL_TOOLS` | Maturity |
|--------------------------------------------------|--------------------------|---------------------------|----------|
| **[Claude Agent](docs/tools/claude-agent.md)**   | Claude Code CLI Agent    | `claude-agent`            | 🟡       |
| **[Codex Agent](docs/tools/codex-agent.md)**     | Codex CLI Agent          | `codex-agent`             | 🟡       |
| **[Copilot Agent](docs/tools/copilot-agent.md)** | GitHub Copilot CLI Agent | `copilot-agent`           | 🟡       |
| **[Gemini Agent](docs/tools/gemini-agent.md)**   | Gemini CLI Agent         | `gemini-agent`            | 🟡       |
| **[Kiro Agent](docs/tools/kiro-agent.md)**       | Kiro CLI Agent           | `kiro-agent`              | 🟡       |

👉 **[See detailed tool documentation](docs/tools/overview.md)**

## Quick Start

### Installation

**Option 1: Go Install** (recommended)
```bash
go install github.com/sammcj/mcp-devtools@HEAD
```

**Option 2: Build from Source**
```bash
git clone https://github.com/sammcj/mcp-devtools.git
cd mcp-devtools
make build
```

**Option 4: Download Release**
Download the latest binary from [releases](https://github.com/sammcj/mcp-devtools/releases) and place in your PATH and remember to check for updates!

### Basic MCP Configuration

**STDIO**

```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools",
      "env": {
        "BRAVE_API_KEY": "This is optional ",
      }
    }
  }
}
```

Replacing `/path/to/mcp-devtools` with your actual binary path (e.g., `/Users/yourname/go/bin/mcp-devtools`).

_Note: The `BRAVE_API_KEY` is optional and only needed if you want to use the Brave Search provider. Other providers like Google, Kagi, SearXNG, and DuckDuckGo are also available. See the [Internet Search documentation](docs/tools/internet_search.md) for more details._

**Streamable HTTP**

```shell
mcp-devtools --transport http --port 18080
```

```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "streamableHttp",
      "url": "http://localhost:18080/http"
    }
  }
}
```

## Transport Options

MCP DevTools supports three transport modes for different use cases:

### STDIO Transport (Default)
**Best for**: Simple, local use with MCP clients like Claude Desktop, Cline, etc.

```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "stdio",
      "command": "/path/to/mcp-devtools",
      "env": {
        "BRAVE_API_KEY": "your-api-key-if-needed"
      }
    }
  }
}
```

### Streamable HTTP Transport
**Best for**: Production deployments, shared use, centralised configuration

```bash
# Basic HTTP mode
mcp-devtools --transport http --port 18080

# With authentication
mcp-devtools --transport http --port 18080 --auth-token mysecrettoken

# With OAuth (see OAuth documentation)
mcp-devtools --transport http --port 18080 --oauth-enabled
```

**Client Configuration:**
```json
{
  "mcpServers": {
    "dev-tools": {
      "type": "streamableHttp",
      "url": "http://localhost:18080/http",
    }
  }
}
```

## Configuration Options

### Environment Variables

All environment variables are optional, but if you want to use specific search providers or document processing features, you may need to provide the the appropriate variables.

**General:**
- `LOG_LEVEL` - Logging level: `debug`, `info`, `warn`, `error` (default: `warn`). Logs are written to `~/.mcp-devtools/logs/mcp-devtools.log` for all transports. Stdio transport uses minimum `warn` level and never logs to stdout/stderr to prevent MCP protocol pollution.
- `LOG_TOOL_ERRORS` - Enable logging of failed tool calls to `~/.mcp-devtools/logs/tool-errors.log` (set to `true` to enable). Logs older than 60 days are automatically removed on server startup.
- `ENABLE_ADDITIONAL_TOOLS` - Comma-separated list to enable security-sensitive tools (e.g. `security,security_override,filesystem,claude-agent,codex-agent,gemini-agent,kiro-agent,process_document,pdf,memory,terraform_documentation,sequential-thinking`)
- `DISABLED_TOOLS` - Comma-separated list of functions to disable (e.g. `think,internet_search`)

**Default Tools:**
- `BRAVE_API_KEY` - Enable Brave Search provider by providing a ([free Brave search API key](https://brave.com/search/api/))
- `GOOGLE_SEARCH_API_KEY` - Enable Google search with API key from [Cloud Console](https://console.cloud.google.com/apis/credentials) (requires Custom Search API to be enabled)
- `GOOGLE_SEARCH_ID` - Google Search Engine ID from [Programmable Search Engine](https://programmablesearchengine.google.com/) (required with `GOOGLE_SEARCH_API_KEY`, select "Search the entire web")
- `KAGI_API_KEY` - Enable Kagi Search provider by providing your [Kagi API key](https://kagi.com/settings?p=api) (requires Kagi subscription)
- `SEARXNG_BASE_URL` - Enable SearXNG search provider by providing the base URL (e.g. `https://searxng.example.com`)
- `CONTEXT7_API_KEY` - Optional Context7 API key for higher rate limits and authentication with package documentation tools
- `MEMORY_FILE_PATH` - Memory storage location (default: `~/.mcp-devtools/`)
- `PACKAGE_COOLDOWN_HOURS` - Hours to wait before recommending newly published packages (default: `72`, set to `0` to disable)
- `PACKAGE_COOLDOWN_ECOSYSTEMS` - Comma-separated ecosystems for cooldown protection (default: `npm`, use `none` to disable)

**Security Configuration:**
- `FILESYSTEM_TOOL_ALLOWED_DIRS` - Colon-separated (Unix) list of allowed directories (only for filesystem tool)

**Document Processing:**
- `DOCLING_PYTHON_PATH` - Python executable path (default: auto-detected)
- `DOCLING_CACHE_ENABLED` - Enable processed document cache (default: `true`)
- `DOCLING_HARDWARE_ACCELERATION` - Hardware acceleration (`auto` (default), `mps`, `cuda`, `cpu`)

### Command-Line Options

- `--transport`, `-t` - Transport type (`stdio`, `sse`, `http`). Default: `stdio`
- `--port` - Port for HTTP transports. Default: `18080`
- `--base-url` - Base URL for HTTP transports. Default: `http://localhost`
- `--auth-token` - Authentication token for HTTP transport

## Architecture

MCP DevTools uses a modular architecture:

- **Tool Registry**: Central registry managing tool discovery and registration
- **Tool Interface**: Standardised interface all tools implement
- **Transport Layer**: Supports STDIO, HTTP, and SSE transports
- **Plugin System**: Easy to add new tools following the interface

Each tool is self-contained and registers automatically when the binary starts.

![](./screenshots/mcp-devtools-1.jpeg)

## Security Framework

MCP DevTools includes a configurable security system that provides multi-layered protection for tools that access files or make HTTP requests.

Important: This feature should be considered in **BETA**, if you find bugs and have solutions please feel free to raise a PR.

### Key Features
- **Access Control**: Prevents tools from accessing sensitive files and domains
- **Content Analysis**: Scans returned content for security threats using pattern matching
- **YAML-Based Configuration**: Easy-to-manage rules with automatic reloading
- **Security Overrides**: Allow bypassing false positives with audit logging
- **Performance Optimised**: Minimal impact when disabled, efficient when enabled

### Built-in Protection
- **Shell Injection Detection**: Command injection, eval execution, backtick commands
- **Data Exfiltration Prevention**: DNS exfiltration, credential theft, keychain access
- **Prompt Injection Mitigation**: "Ignore instructions" attacks, conversation extraction
- **Persistence Mechanism Detection**: Launchctl, systemd, crontab modifications
- **Sensitive File Protection**: SSH keys, AWS credentials, certificates

### Quick Setup
```bash
# Enable security framework and override tool
# You may optionally also add 'security_override' if you want a tool the agent can use to override security warnings
ENABLE_ADDITIONAL_TOOLS="security"
```

Configuration is managed through `~/.mcp-devtools/security.yaml` with sensible defaults.

👉 **[Complete Security Documentation](docs/security.md)**

## Advanced Features

### OAuth 2.0/2.1 Authentication
For production deployments requiring centralised user authentication:

👉 **[Complete OAuth Setup Guide](docs/oauth/README.md)**

Quick example:
```bash
# Browser-based authentication
mcp-devtools --transport http --oauth-browser-auth --oauth-client-id="your-client"

# Resource server mode
mcp-devtools --transport http --oauth-enabled --oauth-issuer="https://auth.example.com"
```

### Proxy Support

All HTTP-based tools automatically support proxy configuration through standard environment variables:

```bash
# HTTPS proxy (preferred)
export HTTPS_PROXY="http://proxy.company.com:8080"

# HTTP proxy (fallback)
export HTTP_PROXY="http://proxy.company.com:8080"

# With authentication
export HTTPS_PROXY="http://username:password@proxy.company.com:8080"

# Run with proxy
./bin/mcp-devtools stdio
```

**Supported tools**: All network-based tools including `package_docs`, `internet_search`, `webfetch`, `github`, `aws_docs`, and others automatically respect proxy settings when configured.

**Security**: Proxy credentials are automatically redacted from logs for security.

### Docker Support

```bash
# Pull the image (main is latest)
docker pull ghcr.io/sammcj/mcp-devtools:main

# Run
docker run -d --name mcp-devtools -p 18080:18080 --restart always ghcr.io/sammcj/mcp-devtools:main

# Run with proxy support
docker run -d --name mcp-devtools -p 18080:18080 \
  -e HTTPS_PROXY="http://proxy.company.com:8080" \
  --restart always ghcr.io/sammcj/mcp-devtools:main
```

## Logging

MCP DevTools maintains two log files in `~/.mcp-devtools/logs/`:

**Application Logs** (`mcp-devtools.log`):
- Contains all application logs at the configured level
- Configure via `LOG_LEVEL` environment variable: `debug`, `info`, `warn`, `error` (default: `warn`)
- **Stdio transport**: Always logs to file (never to stderr to prevent MCP protocol pollution)
- **HTTP/SSE transports**: Logs to file at configured level

**Tool Error Logs** (`tool-errors.log`):
- Failed tool executions with arguments and error details
- Enable via `LOG_TOOL_ERRORS=true` environment variable
- Automatically rotates logs older than 60 days
- Useful for debugging tool calling issues

Example:
```bash
# Enable debug logging for HTTP mode
LOG_LEVEL=debug mcp-devtools --transport http

# Enable tool error logging (works with any transport)
LOG_TOOL_ERRORS=true mcp-devtools
```

## Observability

MCP DevTools includes optional OpenTelemetry integration for distributed tracing and metrics.

![otel screenshot in Jaeger](./docs/otel.png)

**Tracing** - Track tool execution spans with HTTP client operations:
```bash
# Export traces to OTLP endpoint
MCP_OTEL_ENDPOINT=https://your-oltp-endpoint
```

**Metrics** - Opt-in performance and usage metrics:
```bash
# Default enabled traces/metrics when OTEL is enabled
MCP_METRICS_GROUPS=tool,session,security

# Include cache metrics
MCP_METRICS_GROUPS=tool,session,security,cache
```

Available metric groups: `tool` (tool execution), `session` (session lifecycle), `security` (security checks), `cache` (cache operations)

- **[Complete Observability Documentation](docs/observability.md)**
- **[Example Configurations](docs/observability/examples/)**

### Creating New Tools

Want to add your own tools? See the **[Development Guide](docs/creating-new-tools.md)**.

## Getting Help

- **Tool Documentation**: [docs/tools/overview.md](docs/tools/overview.md)
- **Security Framework**: [docs/security.md](docs/security.md)
- **OAuth Setup**: [docs/oauth/README.md](docs/oauth/README.md)
- **Development**: [docs/creating-new-tools.md](docs/creating-new-tools.md)
- **Issues**: [GitHub Issues](https://github.com/sammcj/mcp-devtools/issues), please note that I built this tool for my own use and it is not a commercially supported product, so if you can - please raise a PR instead of an issue.

## Contributing

Contributions welcome! This project follows standard Go development practices and includes comprehensive tests.

Important: See [docs/creating-new-tools.md](docs/creating-new-tools.md) for guidelines on adding new tools.

```bash
# Development setup
git clone https://github.com/sammcj/mcp-devtools.git
cd mcp-devtools
make deps
make test
make build

# Benchmark tool token costs
ENABLE_ADDITIONAL_TOOLS=<your new tool name here> make benchmark-tokens

# Run security checks, see make help
make inspect # launches the MCP inspector tool
```

## Disclaimer

No warranty is provided for this software. Use at your own risk. The author is not responsible for any damages or issues arising from its use.

## License

Apache Public License 2.0 - Copyright 2025 Sam McLeod
