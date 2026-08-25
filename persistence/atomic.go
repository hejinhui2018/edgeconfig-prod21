package persistence

import (
	"fmt"
	"os"
	"path/filepath"
)

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".edgeconfig-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempName := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(mode); err != nil {
		return fmt.Errorf("set temporary file mode: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		// Windows cannot replace an existing destination with Rename.
		backup := path + ".previous"
		_ = os.Remove(backup)
		if moveErr := os.Rename(path, backup); moveErr != nil && !os.IsNotExist(moveErr) {
			return fmt.Errorf("prepare snapshot replacement: %w", moveErr)
		}
		if moveErr := os.Rename(tempName, path); moveErr != nil {
			_ = os.Rename(backup, path)
			return fmt.Errorf("replace snapshot: %w", moveErr)
		}
		_ = os.Remove(backup)
	}
	committed = true
	return syncDirectory(dir)
}

func syncDirectory(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open data directory: %w", err)
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil {
		// Windows commonly rejects directory flushes; file replacement is already atomic there.
		if os.PathSeparator == '\\' {
			return nil
		}
		return fmt.Errorf("sync data directory: %w", err)
	}
	return nil
}
