package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// ReadJSON reads and unmarshals a JSON file from the cache directory.
func ReadJSON(cacheDir, serverHash, filename string, v any) error {
	path := filepath.Join(cacheDir, fmt.Sprintf("%s_%s", serverHash, filename))
	logrus.WithField("path", path).Debug("auth: reading JSON file")

	data, err := os.ReadFile(path)
	if err != nil {
		logrus.WithField("path", path).WithError(err).Debug("auth: failed to read file")
		return err
	}

	return json.Unmarshal(data, v)
}

// WriteJSON marshals and writes a JSON file to the cache directory.
func WriteJSON(cacheDir, serverHash, filename string, v any) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		logrus.WithError(err).Error("auth: failed to create cache directory")
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		logrus.WithError(err).Error("auth: failed to marshal JSON")
		return err
	}

	path := filepath.Join(cacheDir, fmt.Sprintf("%s_%s", serverHash, filename))
	logrus.WithField("path", path).Debug("auth: writing JSON file")

	return os.WriteFile(path, data, 0600)
}

// DeleteFile removes a file from the cache directory.
func DeleteFile(cacheDir, serverHash, filename string) error {
	path := filepath.Join(cacheDir, fmt.Sprintf("%s_%s", serverHash, filename))
	logrus.WithField("path", path).Debug("auth: deleting file")
	return os.Remove(path)
}
