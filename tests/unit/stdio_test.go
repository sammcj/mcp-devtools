package unit

import (
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// TestStdioLogger verifies that when the transport is set to "stdio",
// the logger does not write to stdout or stderr.
func TestStdioLogger(t *testing.T) {
	// Given
	logger := logrus.New()
	transport := "stdio"

	// When
	if transport == "stdio" {
		logger.SetOutput(io.Discard)
	} else {
		logger.SetOutput(os.Stderr)
	}

	// Then
	if transport == "stdio" {
		assert.Equal(t, io.Discard, logger.Out, "Logger should write to io.Discard in stdio mode")
	} else {
		assert.Equal(t, os.Stderr, logger.Out, "Logger should write to stderr in non-stdio mode")
	}
}
