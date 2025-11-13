package code_rename

// LanguageServer represents configuration for a language server
type LanguageServer struct {
	Language string   // Language name (e.g., "go", "python", "typescript")
	Command  string   // Command to execute (e.g., "gopls", "pyright-langserver")
	Args     []string // Arguments to pass to the command
}

// SupportedServers defines LSP servers with rename capability
var SupportedServers = []LanguageServer{
	{Language: "go", Command: "gopls", Args: []string{"serve"}},
	{Language: "typescript", Command: "typescript-language-server", Args: []string{"--stdio"}},
	{Language: "javascript", Command: "typescript-language-server", Args: []string{"--stdio"}},
	{Language: "python", Command: "pyright-langserver", Args: []string{"--stdio"}},
	{Language: "rust", Command: "rust-analyzer", Args: []string{}},
	{Language: "bash", Command: "bash-language-server", Args: []string{"start"}},
	{Language: "sh", Command: "bash-language-server", Args: []string{"start"}},
	{Language: "html", Command: "vscode-html-language-server", Args: []string{"--stdio"}},
	{Language: "css", Command: "vscode-css-language-server", Args: []string{"--stdio"}},
	{Language: "scss", Command: "vscode-css-language-server", Args: []string{"--stdio"}},
	{Language: "less", Command: "vscode-css-language-server", Args: []string{"--stdio"}},
	{Language: "json", Command: "vscode-json-language-server", Args: []string{"--stdio"}},
	{Language: "yaml", Command: "yaml-language-server", Args: []string{"--stdio"}},
	{Language: "c", Command: "clangd", Args: []string{}},
	{Language: "cpp", Command: "clangd", Args: []string{}},
	{Language: "java", Command: "jdtls", Args: []string{}},
	{Language: "swift", Command: "sourcekit-lsp", Args: []string{}},
}

// RenameRequest represents a rename operation request
type RenameRequest struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	NewName  string `json:"new_name"`
	Preview  bool   `json:"preview"`
}

// RenameResult represents the result of a rename operation
// Only returns actionable information - no echo of input parameters
type RenameResult struct {
	FilesModified      int             `json:"files_modified,omitempty"`
	TotalReplacements  int             `json:"total_replacements,omitempty"`
	AffectedFiles      []string        `json:"affected_files,omitempty"`      // List of file paths with change counts
	Applied            bool            `json:"applied,omitempty"`             // Only present when true
	Error              string          `json:"error,omitempty"`               // Only present on failure
	RolledBack         bool            `json:"rolled_back,omitempty"`         // True if rollback was performed
	RollbackSuccessful bool            `json:"rollback_successful,omitempty"` // True if rollback succeeded
	BackupLocation     string          `json:"backup_location,omitempty"`     // Path to backup directory (on failure)
	FilesReverted      []string        `json:"files_reverted,omitempty"`      // Files restored during rollback
	ChangePreview      []ChangeSnippet `json:"change_preview,omitempty"`      // Preview of changes (preview mode only)
}

// ChangeSnippet shows a single change in preview mode
type ChangeSnippet struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Before   string `json:"before"`
	After    string `json:"after"`
}

// RenameTransaction manages atomic rename operations with rollback support
type RenameTransaction struct {
	backupDir     string
	backups       map[string]backupEntry // filePath -> backup info
	modified      []string               // track successfully modified files
	checksums     map[string]string      // filePath -> SHA256 checksum
	transactionID string                 // unique ID for this transaction
}

// backupEntry stores backup information for a file
type backupEntry struct {
	backupPath string // path to backup file
	checksum   string // SHA256 checksum of original
	mode       string // file permissions
}
