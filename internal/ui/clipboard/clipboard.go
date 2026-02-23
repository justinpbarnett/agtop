package clipboard

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/atotto/clipboard"
)

// Write copies text to the system clipboard. It tries the native
// clipboard first (wl-copy, xclip, pbcopy, etc.) then falls back
// to OSC52 for SSH/tmux environments.
func Write(text string) error {
	if err := clipboard.WriteAll(text); err == nil {
		return nil
	}
	return writeOSC52(text)
}

// ReadText reads text from the system clipboard.
func ReadText() (string, error) {
	return clipboard.ReadAll()
}

// ReadImage reads raw PNG image data from the system clipboard.
// It tries wl-paste (Wayland) then xclip (X11). Returns an error if no
// image data is available or neither tool is installed.
func ReadImage() ([]byte, string, error) {
	type tool struct{ args []string }
	tools := []tool{
		{[]string{"wl-paste", "--type", "image/png"}},
		{[]string{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"}},
	}
	for _, t := range tools {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		out, err := exec.CommandContext(ctx, t.args[0], t.args[1:]...).Output()
		cancel()
		if err == nil && len(out) > 0 {
			return out, "image/png", nil
		}
	}
	return nil, "", fmt.Errorf("no image in clipboard")
}

// writeOSC52 writes text to the clipboard using the OSC 52 escape sequence.
func writeOSC52(text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	seq := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
	_, err := os.Stderr.Write([]byte(seq))
	return err
}
