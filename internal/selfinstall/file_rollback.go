package selfinstall

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"
)

type ownedFileState struct {
	exists   bool
	data     []byte
	mode     os.FileMode
	modified time.Time
}

func readOwnedFile(path string) (ownedFileState, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return ownedFileState{}, nil
	}
	if err != nil {
		return ownedFileState{}, err
	}
	if !info.Mode().IsRegular() {
		return ownedFileState{}, fmt.Errorf("shell file is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ownedFileState{}, err
	}
	return ownedFileState{true, data, info.Mode(), info.ModTime()}, nil
}
func (a ownedFileState) equal(b ownedFileState) bool {
	return a.exists == b.exists && (!a.exists || (a.mode == b.mode && a.modified.Equal(b.modified) && bytes.Equal(a.data, b.data)))
}

// mutateOwnedFile snapshots one targeted file and conditionally restores only
// the exact state written by this operation. A later file change is preserved.
func mutateOwnedFile(path string, write func() error) (func() error, error) {
	before, err := readOwnedFile(path)
	if err != nil {
		return nil, err
	}
	writeErr := write()
	after, readErr := readOwnedFile(path)
	if readErr != nil {
		return nil, errors.Join(writeErr, fmt.Errorf("record shell file change: %w", readErr))
	}
	done := false
	undo := func() error {
		if done {
			return nil
		}
		done = true
		if before.equal(after) {
			return nil
		}
		current, err := readOwnedFile(path)
		if err != nil {
			return err
		}
		if !current.equal(after) {
			return nil
		}
		if !before.exists {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		if err := os.WriteFile(path, before.data, before.mode.Perm()); err != nil {
			return err
		}
		if err := os.Chmod(path, before.mode.Perm()); err != nil {
			return err
		}
		return os.Chtimes(path, before.modified, before.modified)
	}
	return undo, writeErr
}
