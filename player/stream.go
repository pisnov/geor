package player

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/lrstanley/go-ytdlp"
)

// InstallYtDlp ensures yt-dlp is installed and returns its executable path.
func InstallYtDlp(ctx context.Context) (string, error) {
	resolved, err := ytdlp.Install(ctx, nil)
	if err != nil {
		return "", err
	}
	return resolved.Executable, nil
}

// GetStreamURL extracts the direct audio URL via yt-dlp -g.
// ffmpeg can then fetch it over HTTP directly, enabling fast HTTP-range seeking.
func GetStreamURL(ctx context.Context, ytdlpBin, url string) (string, error) {
	cmd := exec.CommandContext(ctx, ytdlpBin, "--no-warnings", "-f", "bestaudio", "-g", url)
	cmd.Stderr = io.Discard
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("yt-dlp returned no URL")
}

// buildAtempoFilter builds a pitch-preserving ffmpeg atempo filter chain.
// atempo accepts only 0.5–2.0 per stage; values outside that range are chained.
func buildAtempoFilter(ratio float64) string {
	if ratio == 1.0 {
		return "anull"
	}
	var filters []string
	r := ratio
	for r > 2.0 {
		filters = append(filters, "atempo=2.0")
		r /= 2.0
	}
	for r < 0.5 {
		filters = append(filters, "atempo=0.5")
		r /= 0.5
	}
	filters = append(filters, fmt.Sprintf("atempo=%.4f", r))
	return strings.Join(filters, ",")
}

// StreamAudio fetches audio from a direct HTTP URL via ffmpeg.
// Speed is changed pitch-preservingly via atempo; startPos enables fast seek.
func StreamAudio(ctx context.Context, streamURL string, speed float64, startPos time.Duration) (io.ReadCloser, error) {
	var args []string
	if startPos > 500*time.Millisecond {
		args = append(args, "-ss", fmt.Sprintf("%.3f", startPos.Seconds()))
	}
	args = append(args,
		"-i", streamURL,
		"-vn",
		"-af", buildAtempoFilter(speed),
		"-f", "mp3",
		"-loglevel", "quiet",
		"pipe:1",
	)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = io.Discard
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return out, nil
}
