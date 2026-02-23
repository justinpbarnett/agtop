package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"

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

// writeOSC52 writes text to the clipboard using the OSC 52 escape sequence.
func writeOSC52(text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	seq := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
	_, err := os.Stderr.Write([]byte(seq))
	return err
}
