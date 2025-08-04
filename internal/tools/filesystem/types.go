package filesystem

import (
	"time"
)

// FileSystemRequest represents the base request structure
type FileSystemRequest struct {
	Path string `json:"path"`
}

// ReadFileRequest represents the request for reading a file
type ReadFileRequest struct {
	Path string `json:"path"`
	Head *int   `json:"head,omitempty"` // Read only first N lines
	Tail *int   `json:"tail,omitempty"` // Read only last N lines
}

// ReadMultipleFilesRequest represents the request for reading multiple files
type ReadMultipleFilesRequest struct {
	Paths []string `json:"paths"`
}

// WriteFileRequest represents the request for writing a file
type WriteFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// EditOperation represents a single edit operation
type EditOperation struct {
	OldText string `json:"oldText"`
	NewText string `json:"newText"`
}

// EditFileRequest represents the request for editing a file
type EditFileRequest struct {
	Path   string          `json:"path"`
	Edits  []EditOperation `json:"edits"`
	DryRun bool            `json:"dryRun"`
}

// MoveFileRequest represents the request for moving/renaming files
type MoveFileRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// SearchFilesRequest represents the request for searching files
type SearchFilesRequest struct {
	Path            string   `json:"path"`
	Pattern         string   `json:"pattern"`
	ExcludePatterns []string `json:"excludePatterns"`
}

// ListDirectoryRequest represents the request for listing directory contents
type ListDirectoryRequest struct {
	Path   string `json:"path"`
	SortBy string `json:"sortBy,omitempty"` // "name" or "size"
}

// FileInfo represents file metadata
type FileInfo struct {
	Size        int64     `json:"size"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
	Accessed    time.Time `json:"accessed"`
	IsDirectory bool      `json:"isDirectory"`
	IsFile      bool      `json:"isFile"`
	Permissions string    `json:"permissions"`
}

// DirectoryEntry represents a single directory entry
type DirectoryEntry struct {
	Name     string           `json:"name"`
	Type     string           `json:"type"` // "file" or "directory"
	Size     int64            `json:"size,omitempty"`
	Modified time.Time        `json:"modified,omitempty"`
	Children []DirectoryEntry `json:"children,omitempty"` // Only for directories
}

// AllowedDirectoriesResponse represents the response for listing allowed directories
type AllowedDirectoriesResponse struct {
	Directories []string `json:"directories"`
}
