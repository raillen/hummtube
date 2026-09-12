package app

import (
	"errors"
	"os"
)

const maxLogFileBytes int64 = 2 << 20

// OpenLogFile abre o log persistente do produto com permissões privadas e
// mantém somente uma geração anterior. O limite impede crescimento sem fim em
// máquinas com pouco armazenamento.
func OpenLogFile() (*os.File, error) {
	path, err := LogFilePath()
	if err != nil {
		return nil, err
	}
	if info, statErr := os.Stat(path); statErr == nil && info.Size() >= maxLogFileBytes {
		backupPath := path + ".1"
		if removeErr := os.Remove(backupPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, removeErr
		}
		if renameErr := os.Rename(path, backupPath); renameErr != nil {
			return nil, renameErr
		}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, statErr
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
