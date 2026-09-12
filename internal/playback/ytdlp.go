package playback

import (
	"context"
	"os/exec"
	"strings"
)

// YtdlpInfo returns the yt-dlp path and version without executing shell.
func YtdlpInfo(ctx context.Context) (path, version string, err error) {
	path, err = exec.LookPath("yt-dlp")
	if err != nil {
		return "", "", err
	}
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return path, "", err
	}
	return path, strings.TrimSpace(string(out)), nil
}
