package main

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khanghh/goja-nodejs/require"
)

// rootDirFileLoader returns JavaScript source loader that only allow loading files
// within the specified root directory
func rootDirFileLoader(rootDir string) require.SourceLoader {
	return func(filename string) ([]byte, error) {
		fp := filepath.Join(rootDir, filename)
		relPath, err := filepath.Rel(rootDir, fp)
		if err != nil || strings.HasPrefix(relPath, "..") {
			return nil, require.ErrModuleNotExist
		}
		f, err := os.Open(fp)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, require.ErrModuleNotExist
			}
			return nil, err
		}
		defer f.Close()
		fi, err := f.Stat()
		if err == nil && !fi.IsDir() {
			return io.ReadAll(f)
		}
		return nil, require.ErrModuleNotExist
	}
}

// makeDirIfNotExist recursively creates directories from the given path if does not exist
func makeDirIfNotExist(dirname string) error {
	_, err := os.Stat(dirname)
	if err != nil && os.IsNotExist(err) {
		return os.Mkdir(dirname, 0755)
	}
	return err
}

// randomSource returns a pseudo random value generator.
func randomSource() *rand.Rand {
	bytes := make([]byte, 8)
	seed := time.Now().UnixNano()
	if _, err := crand.Read(bytes); err == nil {
		seed = int64(binary.LittleEndian.Uint64(bytes))
	}

	src := rand.NewSource(seed)
	return rand.New(src)
}
