package code_rename

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sammcj/mcp-devtools/internal/security"
	"github.com/sirupsen/logrus"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// readWriteCloser combines separate reader and writer into ReadWriteCloser
type readWriteCloser struct {
	reader io.ReadCloser
	writer io.WriteCloser
}

func (rwc *readWriteCloser) Read(p []byte) (n int, err error) {
	return rwc.reader.Read(p)
}

func (rwc *readWriteCloser) Write(p []byte) (n int, err error) {
	return rwc.writer.Write(p)
}

func (rwc *readWriteCloser) Close() error {
	err1 := rwc.reader.Close()
	err2 := rwc.writer.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// LSPClient wraps an LSP server connection
type LSPClient struct {
	server       *LanguageServer
	conn         jsonrpc2.Conn
	cmd          *exec.Cmd
	rootURI      string
	logger       *logrus.Logger
	serverCancel context.CancelFunc
	openDocs     map[string]bool
	docVersions  map[string]int32 // Track document versions for didChange
	docMu        sync.Mutex
}

// NewLSPClient creates and initialises a new LSP client
func NewLSPClient(ctx context.Context, logger *logrus.Logger, server *LanguageServer, filePath string) (*LSPClient, error) {
	// Determine root URI (workspace root)
	rootPath, err := findWorkspaceRoot(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to find workspace root: %w", err)
	}

	rootURI := pathToURI(rootPath)

	// Start LSP server process with background context
	// Use background context so server lifetime isn't tied to MCP tool execution timeout
	serverCtx, serverCancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(serverCtx, server.Command, server.Args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		serverCancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		serverCancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		serverCancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		serverCancel()
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Register process with tracker
	tracker := GetProcessTracker(logger)
	tracker.Register(cmd.Process.Pid, server.Command, cmd.Process)

	// Log stderr output in background for debugging
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				logger.WithField("server", server.Command).Debugf("LSP stderr: %s", string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// Create JSON-RPC connection
	// Combine stdout (reader) and stdin (writer) into a single ReadWriteCloser
	stream := &readWriteCloser{reader: stdout, writer: stdin}
	conn := jsonrpc2.NewConn(jsonrpc2.NewStream(stream))

	client := &LSPClient{
		server:       server,
		conn:         conn,
		cmd:          cmd,
		rootURI:      rootURI,
		logger:       logger,
		serverCancel: serverCancel,
		openDocs:     make(map[string]bool),
		docVersions:  make(map[string]int32),
	}

	// Start the message pump with a handler for server->client messages
	// Use background context so message pump lifetime isn't tied to MCP tool execution timeout
	handler := func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		// Log server->client requests/notifications for debugging
		logger.WithFields(logrus.Fields{
			"method": req.Method(),
			"server": server.Command,
		}).Debug("LSP server message")

		// Check if this is a Call (request with ID) or Notification (no reply expected)
		if _, isCall := req.(*jsonrpc2.Call); isCall {
			// This is a request, send method not found since we don't handle server->client requests
			return reply(ctx, nil, jsonrpc2.NewError(jsonrpc2.MethodNotFound, "method not supported"))
		}

		// This is a notification, don't reply
		return nil
	}
	conn.Go(context.Background(), handler)

	// Initialise the LSP connection
	if err := client.initialize(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to initialise LSP: %w", err)
	}

	return client, nil
}

// initialise sends the initialise request to the LSP server
func (c *LSPClient) initialize(_ context.Context) error {
	// Use timeout context derived from Background() for consistency with message pump
	initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	initParams := &protocol.InitializeParams{
		ProcessID: int32(os.Getpid()),
		RootURI:   protocol.DocumentURI(c.rootURI),
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Rename: &protocol.RenameClientCapabilities{
					PrepareSupport: true,
				},
			},
		},
	}

	var result protocol.InitializeResult
	if _, err := c.conn.Call(initCtx, "initialise", initParams, &result); err != nil {
		return fmt.Errorf("initialise failed: %w", err)
	}

	// Send initialised notification
	if err := c.conn.Notify(initCtx, "initialised", &protocol.InitializedParams{}); err != nil {
		return fmt.Errorf("initialised notification failed: %w", err)
	}

	c.logger.WithField("server", c.server.Command).Debug("LSP server initialised")
	return nil
}

// openDocument sends textDocument/didOpen notification to inform server about the file
func (c *LSPClient) openDocument(ctx context.Context, filePath string) error {
	fileURI := pathToURI(filePath)

	// Check if document is already open to avoid protocol violations
	c.docMu.Lock()
	alreadyOpen := c.openDocs[fileURI]
	c.docMu.Unlock()

	if alreadyOpen {
		c.logger.WithField("uri", fileURI).Debug("Document already open, syncing instead")
		// Document already open - sync it to ensure LSP has latest content
		return c.SyncDocument(ctx, filePath)
	}

	c.docMu.Lock()
	defer c.docMu.Unlock()

	// Security: Check file access permission
	if err := security.CheckFileAccess(filePath); err != nil {
		return fmt.Errorf("access denied: %w", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	params := &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        protocol.DocumentURI(fileURI),
			LanguageID: protocol.LanguageIdentifier(c.server.Language),
			Version:    1,
			Text:       string(content),
		},
	}

	if err := c.conn.Notify(ctx, "textDocument/didOpen", params); err != nil {
		return err
	}

	// Mark document as open and set initial version
	c.openDocs[fileURI] = true
	c.docVersions[fileURI] = 1
	c.logger.WithField("uri", fileURI).Debug("Document opened")

	return nil
}

// SyncDocument sends textDocument/didChange to update the LSP server's view of a file
// This should be called after modifying files to keep the LSP server in sync
func (c *LSPClient) SyncDocument(ctx context.Context, filePath string) error {
	fileURI := pathToURI(filePath)

	c.docMu.Lock()
	defer c.docMu.Unlock()

	// Only sync if document is open
	if !c.openDocs[fileURI] {
		c.logger.WithField("uri", fileURI).Debug("Document not open, skipping sync")
		return nil
	}

	// Security: Check file access permission
	if err := security.CheckFileAccess(filePath); err != nil {
		return fmt.Errorf("access denied: %w", err)
	}

	// Read current file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Increment version
	c.docVersions[fileURI]++
	version := c.docVersions[fileURI]

	// Send didChange with full document content
	params := &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(fileURI),
			},
			Version: version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{
				// Full document sync - replace entire content
				Text: string(content),
			},
		},
	}

	if err := c.conn.Notify(ctx, "textDocument/didChange", params); err != nil {
		return fmt.Errorf("failed to send didChange: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"uri":     fileURI,
		"version": version,
	}).Debug("Document synchronised with LSP server")

	return nil
}

// PrepareRename calls textDocument/prepareRename to get the current symbol
func (c *LSPClient) PrepareRename(ctx context.Context, filePath string, line, column int) (string, error) {
	// First, open the document
	if err := c.openDocument(ctx, filePath); err != nil {
		return "", fmt.Errorf("failed to open document: %w", err)
	}

	fileURI := pathToURI(filePath)

	params := &protocol.PrepareRenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(fileURI),
			},
			Position: protocol.Position{
				Line:      uint32(line - 1),   // LSP uses 0-based lines
				Character: uint32(column - 1), // LSP uses 0-based columns
			},
		},
	}

	// Use timeout context for LSP call
	callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result json.RawMessage
	if _, err := c.conn.Call(callCtx, "textDocument/prepareRename", params, &result); err != nil {
		return "", fmt.Errorf("prepareRename failed: %w", err)
	}

	// LSP spec: result can be Range | { range: Range, placeholder: string } | null
	// Try to parse as { range, placeholder } first
	var rangeWithPlaceholder struct {
		Range       *protocol.Range `json:"range"`
		Placeholder string          `json:"placeholder"`
	}

	if err := json.Unmarshal(result, &rangeWithPlaceholder); err == nil && rangeWithPlaceholder.Placeholder != "" {
		return rangeWithPlaceholder.Placeholder, nil
	}

	// Try to parse as Range and extract text from file
	var rangeOnly protocol.Range
	if err := json.Unmarshal(result, &rangeOnly); err == nil {
		// Read file and extract symbol text from range
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file for symbol extraction: %w", err)
		}
		lines := strings.Split(string(content), "\n")
		symbolText := extractTextFromRange(lines, rangeOnly)
		if symbolText != "" {
			return symbolText, nil
		}
	}

	// Result is null or unparseable - symbol not renameable
	return "", fmt.Errorf("symbol at position is not renameable")
}

// Rename performs the actual rename operation
func (c *LSPClient) Rename(ctx context.Context, filePath string, line, column int, newName string) (*protocol.WorkspaceEdit, error) {
	// Ensure document is open
	if err := c.openDocument(ctx, filePath); err != nil {
		return nil, fmt.Errorf("failed to open document: %w", err)
	}

	fileURI := pathToURI(filePath)

	params := &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: protocol.DocumentURI(fileURI),
			},
			Position: protocol.Position{
				Line:      uint32(line - 1),   // LSP uses 0-based lines
				Character: uint32(column - 1), // LSP uses 0-based columns
			},
		},
		NewName: newName,
	}

	// Use timeout context for LSP call (longer timeout for potentially large refactorings)
	callCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var result protocol.WorkspaceEdit
	if _, err := c.conn.Call(callCtx, "textDocument/rename", params, &result); err != nil {
		return nil, fmt.Errorf("rename failed: %w", err)
	}

	return &result, nil
}

// Close shuts down the LSP client and server with panic recovery
func (c *LSPClient) Close() (err error) {
	// Panic recovery to ensure cleanup happens even if something goes wrong
	defer func() {
		if r := recover(); r != nil {
			c.logger.WithField("panic", r).Error("Panic during LSP client close, attempting cleanup")
			// Cancel context if not already done
			if c.serverCancel != nil {
				c.serverCancel()
			}
			// Still try to kill the process
			if c.cmd != nil && c.cmd.Process != nil {
				_ = c.cmd.Process.Kill()
				tracker := GetProcessTracker(c.logger)
				tracker.Deregister(c.cmd.Process.Pid)
			}
		}
	}()

	pid := 0
	if c.cmd != nil && c.cmd.Process != nil {
		pid = c.cmd.Process.Pid
	}

	if c.conn != nil {
		// Send shutdown request
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, _ = c.conn.Call(ctx, "shutdown", nil, nil)
		_ = c.conn.Notify(ctx, "exit", nil)
		_ = c.conn.Close()
	}

	// Cancel the server context to signal graceful shutdown
	if c.serverCancel != nil {
		c.serverCancel()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		// Wait briefly for graceful shutdown
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		select {
		case <-time.After(1 * time.Second):
			// Timeout waiting for graceful shutdown, force kill
			_ = c.cmd.Process.Kill()
			_ = c.cmd.Wait()
		case <-done:
			// Process exited gracefully
		}

		// Deregister from tracker
		tracker := GetProcessTracker(c.logger)
		tracker.Deregister(pid)
	}

	return nil
}

// findWorkspaceRoot attempts to find the workspace root
// It looks for common markers like .git, go.mod, package.json, etc.
func findWorkspaceRoot(filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(absPath)

	// Walk up the directory tree looking for workspace markers
	markers := []string{".git", "go.mod", "package.json", "Cargo.toml", "pyproject.toml", ".vscode"}

	for {
		for _, marker := range markers {
			markerPath := filepath.Join(dir, marker)
			if _, err := os.Stat(markerPath); err == nil {
				return dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding a marker, use the file's directory
			return filepath.Dir(absPath), nil
		}
		dir = parent
	}
}

// pathToURI converts a file path to a URI
func pathToURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Normalise path separators for URI
	absPath = filepath.ToSlash(absPath)

	// Create proper file:// URI
	u := &url.URL{
		Scheme: "file",
		Path:   absPath,
	}

	return u.String()
}

// uriToPath converts a URI to a file path
func uriToPath(uriStr string) string {
	u := uri.New(uriStr)
	return u.Filename()
}

// convertWorkspaceEdit converts LSP WorkspaceEdit to our RenameResult format
// Only returns actionable information - no echo of input parameters
func convertWorkspaceEdit(edit *protocol.WorkspaceEdit, preview bool) (*RenameResult, error) {
	if edit == nil {
		return &RenameResult{
			Error: "LSP server returned nil workspace edit - symbol may not exist or rename not supported at this location",
		}, nil
	}

	// LSP can return changes in either Changes or DocumentChanges format
	// Modern servers (like gopls) prefer DocumentChanges
	if len(edit.Changes) == 0 && len(edit.DocumentChanges) == 0 {
		return &RenameResult{
			Error: "no changes found - verify the line/column position points to a renameable symbol",
		}, nil
	}

	result := &RenameResult{}

	// Only set Applied field when changes are actually applied
	if !preview {
		result.Applied = true
	}

	totalReplacements := 0
	affectedFiles := make(map[string]int) // Map file paths to change counts
	changePreview := []ChangeSnippet{}    // Preview snippets

	// Process legacy Changes format (map of URI -> TextEdit[])
	for uriStr, textEdits := range edit.Changes {
		filePath := uriToPath(string(uriStr))

		// Security: Check file access permission
		if err := security.CheckFileAccess(filePath); err != nil {
			return nil, fmt.Errorf("access denied for %s: %w", filePath, err)
		}

		changeCount := len(textEdits)
		totalReplacements += changeCount
		affectedFiles[filePath] = changeCount

		// Extract preview snippets if in preview mode
		if preview {
			snippets := extractChangeSnippets(filePath, textEdits, 5)
			changePreview = append(changePreview, snippets...)
		}
	}

	// Process modern DocumentChanges format (array of TextDocumentEdit)
	for _, textDocEdit := range edit.DocumentChanges {
		filePath := uriToPath(string(textDocEdit.TextDocument.URI))

		// Security: Check file access permission
		if err := security.CheckFileAccess(filePath); err != nil {
			return nil, fmt.Errorf("access denied for %s: %w", filePath, err)
		}

		changeCount := len(textDocEdit.Edits)
		totalReplacements += changeCount
		affectedFiles[filePath] += changeCount // Accumulate if file appears in both formats

		// Extract preview snippets if in preview mode
		if preview {
			snippets := extractChangeSnippets(filePath, textDocEdit.Edits, 5)
			changePreview = append(changePreview, snippets...)
		}
	}

	// Limit total preview snippets to 50
	if len(changePreview) > 50 {
		changePreview = changePreview[:50]
	}

	// Convert map to slice
	affectedFilesList := []string{}
	for filePath, count := range affectedFiles {
		affectedFilesList = append(affectedFilesList, fmt.Sprintf("%s (%d changes)", filePath, count))
	}

	result.FilesModified = len(affectedFiles)
	result.TotalReplacements = totalReplacements
	result.AffectedFiles = affectedFilesList

	if preview && len(changePreview) > 0 {
		result.ChangePreview = changePreview
	}

	return result, nil
}

// extractChangeSnippets extracts preview snippets from text edits
// Limited to maxSnippets per file
func extractChangeSnippets(filePath string, edits []protocol.TextEdit, maxSnippets int) []ChangeSnippet {
	snippets := []ChangeSnippet{}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return snippets
	}

	lines := strings.Split(string(content), "\n")

	// Process up to maxSnippets edits
	count := 0
	for _, edit := range edits {
		if count >= maxSnippets {
			break
		}

		lineNum := int(edit.Range.Start.Line) + 1 // Convert to 1-based
		if lineNum > 0 && lineNum <= len(lines) {
			before := lines[lineNum-1]
			// Apply edit to show what it would become
			after := applyTextEdit([]string{before}, protocol.TextEdit{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: edit.Range.Start.Character},
					End:   protocol.Position{Line: 0, Character: edit.Range.End.Character},
				},
				NewText: edit.NewText,
			})[0]

			snippets = append(snippets, ChangeSnippet{
				FilePath: filePath,
				Line:     lineNum,
				Before:   strings.TrimSpace(before),
				After:    strings.TrimSpace(after),
			})
			count++
		}
	}

	return snippets
}

// extractTextFromRange extracts text from file lines using LSP range
func extractTextFromRange(lines []string, r protocol.Range) string {
	startLine := int(r.Start.Line)
	startChar := int(r.Start.Character)
	endLine := int(r.End.Line)
	endChar := int(r.End.Character)

	if startLine < 0 || startLine >= len(lines) {
		return ""
	}

	if startLine == endLine {
		line := lines[startLine]
		if endChar > len(line) {
			endChar = len(line)
		}
		if startChar > len(line) {
			return ""
		}
		return line[startChar:endChar]
	}

	// Multi-line range (rare for rename)
	var result strings.Builder
	result.WriteString(lines[startLine][startChar:])
	for i := startLine + 1; i < endLine; i++ {
		result.WriteString("\n")
		result.WriteString(lines[i])
	}
	result.WriteString("\n")
	if endLine < len(lines) {
		result.WriteString(lines[endLine][:endChar])
	}

	return result.String()
}
