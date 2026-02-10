package api

import (
	"os"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestProcessExists_CurrentProcess(t *testing.T) {
	if !processExists(os.Getpid()) {
		t.Error("current process should exist")
	}
}

func TestProcessExists_InvalidPID(t *testing.T) {
	if processExists(0) {
		t.Error("pid 0 should not exist")
	}
	if processExists(-1) {
		t.Error("negative pid should not exist")
	}
}

func TestProcessExists_DeadPID(t *testing.T) {
	// Use a very high PID unlikely to exist
	if processExists(4194304) {
		t.Skip("pid 4194304 unexpectedly exists")
	}
}

func TestIsLocalHost_Localhost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{"localhost", "localhost", true},
		{"LOCALHOST", "LOCALHOST", true},
		{"127.0.0.1", "127.0.0.1", true},
		{"empty", "", false},
		{"spaces", "   ", false},
		{"whitespace localhost", " localhost ", true},
		{"unknown host", "remote-server.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalHost(tt.host)
			if got != tt.want {
				t.Errorf("isLocalHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestIsLocalHost_CurrentHostname(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		t.Skip("cannot determine hostname")
	}
	if !isLocalHost(hostname) {
		t.Errorf("isLocalHost(%q) = false, want true (current hostname)", hostname)
	}
}

func TestIsProvablyOrphan_Nil(t *testing.T) {
	if isProvablyOrphan(nil) {
		t.Error("nil record should not be orphan")
	}
}

func TestIsProvablyOrphan_NilPID(t *testing.T) {
	rec := &core.RunningWorkflowRecord{LockHolderHost: "localhost"}
	if isProvablyOrphan(rec) {
		t.Error("nil PID should not be orphan")
	}
}

func TestIsProvablyOrphan_RemoteHost(t *testing.T) {
	pid := 12345
	rec := &core.RunningWorkflowRecord{
		LockHolderPID:  &pid,
		LockHolderHost: "remote-server.example.com",
	}
	if isProvablyOrphan(rec) {
		t.Error("remote host should not be provably orphan")
	}
}

func TestIsProvablyOrphan_LocalDeadProcess(t *testing.T) {
	deadPID := 4194304 // very high PID, unlikely to exist
	rec := &core.RunningWorkflowRecord{
		LockHolderPID:  &deadPID,
		LockHolderHost: "localhost",
	}
	if !isProvablyOrphan(rec) {
		t.Skip("pid 4194304 unexpectedly exists")
	}
}

func TestIsProvablyOrphan_LocalAliveProcess(t *testing.T) {
	myPID := os.Getpid()
	rec := &core.RunningWorkflowRecord{
		LockHolderPID:  &myPID,
		LockHolderHost: "localhost",
	}
	if isProvablyOrphan(rec) {
		t.Error("current process should not be orphan")
	}
}

func TestIsOrphanInThisProcess_Nil(t *testing.T) {
	if isOrphanInThisProcess(nil) {
		t.Error("nil record should not be orphan in this process")
	}
}

func TestIsOrphanInThisProcess_NilPID(t *testing.T) {
	rec := &core.RunningWorkflowRecord{LockHolderHost: "localhost"}
	if isOrphanInThisProcess(rec) {
		t.Error("nil PID should not be orphan in this process")
	}
}

func TestIsOrphanInThisProcess_RemoteHost(t *testing.T) {
	myPID := os.Getpid()
	rec := &core.RunningWorkflowRecord{
		LockHolderPID:  &myPID,
		LockHolderHost: "remote-server.example.com",
	}
	if isOrphanInThisProcess(rec) {
		t.Error("remote host should not be orphan in this process")
	}
}

func TestIsOrphanInThisProcess_SamePID(t *testing.T) {
	myPID := os.Getpid()
	rec := &core.RunningWorkflowRecord{
		LockHolderPID:  &myPID,
		LockHolderHost: "localhost",
	}
	if !isOrphanInThisProcess(rec) {
		t.Error("current PID on localhost should be orphan in this process")
	}
}

func TestIsOrphanInThisProcess_DifferentPID(t *testing.T) {
	otherPID := os.Getpid() + 99999
	rec := &core.RunningWorkflowRecord{
		LockHolderPID:  &otherPID,
		LockHolderHost: "localhost",
	}
	if isOrphanInThisProcess(rec) {
		t.Error("different PID should not be orphan in this process")
	}
}
