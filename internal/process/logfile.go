package process

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"
)

const followPollInterval = 100 * time.Millisecond

// FollowReader wraps an *os.File and implements io.ReadCloser with tail-follow
// semantics. When the underlying file returns io.EOF, the reader polls for new
// data until its context is cancelled. This allows reading a file that is being
// actively written to by another process (like tail -f).
type FollowReader struct {
	file   *os.File
	ctx    context.Context
	cancel context.CancelFunc
}

// NewFollowReader creates a FollowReader that reads from f, polling for new
// data on EOF until ctx is cancelled.
func NewFollowReader(ctx context.Context, f *os.File) *FollowReader {
	ctx, cancel := context.WithCancel(ctx)
	return &FollowReader{
		file:   f,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *FollowReader) Read(p []byte) (int, error) {
	for {
		n, err := r.file.Read(p)
		if n > 0 {
			return n, nil
		}
		if err != io.EOF {
			return 0, err
		}
		// At EOF — check if we should stop or poll for more data
		select {
		case <-r.ctx.Done():
			return 0, io.EOF
		default:
		}
		time.Sleep(followPollInterval)
	}
}

func (r *FollowReader) Close() error {
	r.cancel()
	return r.file.Close()
}

// LogFiles manages the stdout and stderr log files for a single run.
type LogFiles struct {
	stdoutPath string
	stderrPath string
	stdout     *os.File
	stderr     *os.File
}

// CreateLogFiles creates new log files for a run in the given sessions directory.
// Files are opened for writing (append mode).
func CreateLogFiles(sessionsDir, runID string) (*LogFiles, error) {
	stdoutPath := filepath.Join(sessionsDir, runID+".stdout")
	stderrPath := filepath.Join(sessionsDir, runID+".stderr")

	stdoutF, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	stderrF, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		stdoutF.Close()
		return nil, err
	}

	return &LogFiles{
		stdoutPath: stdoutPath,
		stderrPath: stderrPath,
		stdout:     stdoutF,
		stderr:     stderrF,
	}, nil
}

// OpenLogFiles opens existing log files for reading.
func OpenLogFiles(stdoutPath, stderrPath string) (*LogFiles, error) {
	stdoutF, err := os.Open(stdoutPath)
	if err != nil {
		return nil, err
	}

	stderrF, err := os.Open(stderrPath)
	if err != nil {
		stdoutF.Close()
		return nil, err
	}

	return &LogFiles{
		stdoutPath: stdoutPath,
		stderrPath: stderrPath,
		stdout:     stdoutF,
		stderr:     stderrF,
	}, nil
}

func (lf *LogFiles) StdoutPath() string     { return lf.stdoutPath }
func (lf *LogFiles) StderrPath() string     { return lf.stderrPath }
func (lf *LogFiles) StdoutWriter() *os.File { return lf.stdout }
func (lf *LogFiles) StderrWriter() *os.File { return lf.stderr }

func (lf *LogFiles) Close() error {
	var firstErr error
	if lf.stdout != nil {
		if err := lf.stdout.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if lf.stderr != nil {
		if err := lf.stderr.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RemoveFiles deletes both log files from disk.
func RemoveLogFiles(stdoutPath, stderrPath string) {
	os.Remove(stdoutPath)
	os.Remove(stderrPath)
}
