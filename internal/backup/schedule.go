package backup

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

func ShouldBackup(last time.Time, interval time.Duration) bool {
	if last.IsZero() {
		return true
	}
	return time.Since(last) >= interval
}

func Retain(dir string, keep int) error {
	if keep <= 0 {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var backs []os.DirEntry
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".ntbackup" {
			backs = append(backs, e)
		}
	}
	if len(backs) <= keep {
		return nil
	}
	type fi struct {
		name string
		mod  time.Time
	}
	var files []fi
	for _, e := range backs {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fi{e.Name(), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for i := 0; i < len(files)-keep; i++ {
		_ = os.Remove(filepath.Join(dir, files[i].name))
	}
	return nil
}
