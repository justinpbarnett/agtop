package server

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
)

type RunningServer struct {
	Port    int
	Cmd     *exec.Cmd
	WorkDir string
	cancel  context.CancelFunc
	done    chan struct{}
}

type DevServerManager struct {
	command      string
	basePort     int
	portStrategy string
	mu           sync.Mutex
	servers      map[string]*RunningServer
}

func NewDevServerManager(cfg config.DevServerConfig) *DevServerManager {
	return &DevServerManager{
		command:      cfg.Command,
		basePort:     cfg.BasePort,
		portStrategy: cfg.PortStrategy,
		servers:      make(map[string]*RunningServer),
	}
}

func (m *DevServerManager) Start(runID string, workDir string) (int, error) {
	if m.command == "" {
		return 0, nil
	}

	m.mu.Lock()
	if _, exists := m.servers[runID]; exists {
		m.mu.Unlock()
		return 0, fmt.Errorf("dev server already running for run %s", runID)
	}
	m.mu.Unlock()

	port := m.ComputePort(runID)
	parts := strings.Fields(m.command)
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty dev server command")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		cancel()
		return 0, fmt.Errorf("start dev server: %w", err)
	}

	done := make(chan struct{})
	srv := &RunningServer{
		Port:    port,
		Cmd:     cmd,
		WorkDir: workDir,
		cancel:  cancel,
		done:    done,
	}

	m.mu.Lock()
	m.servers[runID] = srv
	m.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		close(done)
		m.mu.Lock()
		delete(m.servers, runID)
		m.mu.Unlock()
	}()

	return port, nil
}

func (m *DevServerManager) Stop(runID string) error {
	m.mu.Lock()
	srv, ok := m.servers[runID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	if srv.Cmd.Process == nil {
		srv.cancel()
		return nil
	}

	// SIGTERM first
	_ = srv.Cmd.Process.Signal(syscall.SIGTERM)

	select {
	case <-srv.done:
	case <-time.After(5 * time.Second):
		_ = srv.Cmd.Process.Signal(syscall.SIGKILL)
		<-srv.done
	}

	srv.cancel()
	return nil
}

func (m *DevServerManager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.servers))
	for id := range m.servers {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		_ = m.Stop(id)
	}
}

func (m *DevServerManager) ComputePort(runID string) int {
	h := fnv.New32a()
	h.Write([]byte(runID))
	return m.basePort + int(h.Sum32()%100)
}

func (m *DevServerManager) Port(runID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	srv, ok := m.servers[runID]
	if !ok {
		return 0
	}
	return srv.Port
}
