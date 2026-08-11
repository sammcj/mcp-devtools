package code_rename

import (
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// documentTextEdits pairs a document URI with the text edits targeting it.
type documentTextEdits struct {
	URI   uri.URI
	Edits []protocol.TextEdit
}

// textDocumentEdits narrows WorkspaceEdit.DocumentChanges to the text document
// edits a rename can act on. The union also carries create, rename and delete
// file operations, which a textDocument/rename response does not produce and
// which this tool does not perform.
func textDocumentEdits(edit *protocol.WorkspaceEdit) []documentTextEdits {
	if edit == nil {
		return nil
	}

	docEdits := make([]documentTextEdits, 0, len(edit.DocumentChanges))
	for _, change := range edit.DocumentChanges {
		textDocEdit, ok := change.(*protocol.TextDocumentEdit)
		if !ok {
			continue
		}

		docEdits = append(docEdits, documentTextEdits{
			URI:   textDocEdit.TextDocument.URI,
			Edits: plainTextEdits(textDocEdit.Edits),
		})
	}

	return docEdits
}

// plainTextEdits narrows the edit union to plain replacements. Annotated edits
// carry an embedded TextEdit and are unwrapped; snippet edits describe an
// interactive template rather than a literal replacement, so they are skipped.
func plainTextEdits(elements []protocol.TextDocumentEditElement) []protocol.TextEdit {
	edits := make([]protocol.TextEdit, 0, len(elements))
	for _, element := range elements {
		switch edit := element.(type) {
		case *protocol.TextEdit:
			edits = append(edits, *edit)
		case *protocol.AnnotatedTextEdit:
			edits = append(edits, edit.TextEdit)
		}
	}

	return edits
}
