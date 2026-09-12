//go:build unix

package playback

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// protectProcessTree mantém yt-dlp e runtimes auxiliares no mesmo grupo para
// que o cancelamento não deixe processos descendentes em execução.
func protectProcessTree(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.WaitDelay = 2 * time.Second
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("encerrar grupo do extractor: %w", err)
		}
		return nil
	}
}
