package server

import (
	"testing"

	"github.com/justinpbarnett/agtop/internal/config"
)

func TestComputePortDeterministic(t *testing.T) {
	dm := NewDevServerManager(config.DevServerConfig{
		Command:      "echo test",
		BasePort:     3100,
		PortStrategy: "hash",
	})

	port1 := dm.ComputePort("001")
	port2 := dm.ComputePort("001")
	if port1 != port2 {
		t.Errorf("same runID produced different ports: %d vs %d", port1, port2)
	}

	if port1 < 3100 || port1 >= 3200 {
		t.Errorf("port %d outside range [3100, 3200)", port1)
	}
}

func TestComputePortDifferentIDs(t *testing.T) {
	dm := NewDevServerManager(config.DevServerConfig{
		Command:      "echo test",
		BasePort:     3100,
		PortStrategy: "hash",
	})

	// Different IDs should (usually) produce different ports.
	// We can't guarantee it due to hash collisions, but we verify the range.
	port1 := dm.ComputePort("001")
	port2 := dm.ComputePort("999")

	if port1 < 3100 || port1 >= 3200 {
		t.Errorf("port1 %d outside range", port1)
	}
	if port2 < 3100 || port2 >= 3200 {
		t.Errorf("port2 %d outside range", port2)
	}
}

func TestStartEmptyCommand(t *testing.T) {
	dm := NewDevServerManager(config.DevServerConfig{
		BasePort: 3100,
	})

	port, err := dm.Start("001", t.TempDir())
	if err != nil {
		t.Fatalf("Start with empty command: %v", err)
	}
	if port != 0 {
		t.Errorf("expected port 0 for empty command, got %d", port)
	}
}

func TestStartStop(t *testing.T) {
	dm := NewDevServerManager(config.DevServerConfig{
		Command:      "sleep 60",
		BasePort:     3100,
		PortStrategy: "hash",
	})

	port, err := dm.Start("test-run", t.TempDir())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if port == 0 {
		t.Fatal("expected non-zero port")
	}

	if dm.Port("test-run") != port {
		t.Errorf("Port() = %d, want %d", dm.Port("test-run"), port)
	}

	if err := dm.Stop("test-run"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if dm.Port("test-run") != 0 {
		t.Error("Port() should be 0 after stop")
	}
}

func TestStopAll(t *testing.T) {
	dm := NewDevServerManager(config.DevServerConfig{
		Command:      "sleep 60",
		BasePort:     3100,
		PortStrategy: "hash",
	})

	_, _ = dm.Start("run-a", t.TempDir())
	_, _ = dm.Start("run-b", t.TempDir())

	dm.StopAll()

	if dm.Port("run-a") != 0 || dm.Port("run-b") != 0 {
		t.Error("StopAll did not stop all servers")
	}
}

func TestStopNonexistent(t *testing.T) {
	dm := NewDevServerManager(config.DevServerConfig{
		Command:  "sleep 60",
		BasePort: 3100,
	})

	if err := dm.Stop("nonexistent"); err != nil {
		t.Fatalf("Stop nonexistent should not error: %v", err)
	}
}
