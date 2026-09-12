//go:build windows

package playback

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// O Windows não possui grupos POSIX. taskkill /T encerra também os processos
// auxiliares iniciados pelo yt-dlp; Process.Kill permanece como fallback.
func protectProcessTree(command *exec.Cmd) {
	command.WaitDelay = 2 * time.Second
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		output, treeErr := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid)).CombinedOutput()
		if treeErr == nil {
			return nil
		}
		killErr := command.Process.Kill()
		if killErr == nil || errors.Is(killErr, os.ErrProcessDone) {
			return nil
		}
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = treeErr.Error()
		}
		return fmt.Errorf("encerrar árvore do extractor (%s): %w", detail, killErr)
	}
}
