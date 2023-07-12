package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khanghh/goja-nodejs/require"
)

func TestRootDirFileLoader(t *testing.T) {
	// Create a temporary directory.
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file within the temp directory.
	testFilePath := filepath.Join(tmpDir, "test.txt")
	err = ioutil.WriteFile(testFilePath, []byte("Test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Create a file outside of the temp directory.
	nonRootFilePath := filepath.Join(".", "..", "test_outside_root.txt")
	err = ioutil.WriteFile(nonRootFilePath, []byte("Outside root content"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(nonRootFilePath)

	// Initialize a rootDirFileLoader with the temporary directory.
	sourceLoader := rootDirFileLoader(tmpDir)

	t.Run("Load valid file", func(t *testing.T) {
		data, err := sourceLoader("test.txt")
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "Test content" {
			t.Errorf("expected 'Test content', got '%s'", data)
		}
	})

	t.Run("Load nonexistent file", func(t *testing.T) {
		_, err := sourceLoader("nonexistent.txt")
		if err == nil || !strings.Contains(err.Error(), require.ErrModuleNotExist.Error()) {
			t.Errorf("expected error '%s', got '%v'", require.ErrModuleNotExist, err)
		}
	})

	t.Run("Load file outside root directory", func(t *testing.T) {
		_, err := sourceLoader(nonRootFilePath)
		if err == nil || !strings.Contains(err.Error(), require.ErrModuleNotExist.Error()) {
			t.Errorf("expected error '%s', got '%v'", require.ErrModuleNotExist, err)
		}
	})
}
