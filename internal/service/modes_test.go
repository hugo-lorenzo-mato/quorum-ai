package service_test

import (
	"context"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/service"
	"github.com/hugo-lorenzo-mato/quorum-ai/internal/testutil"
)

func TestExecutionMode_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mode    service.ExecutionMode
		wantErr bool
	}{
		{
			name:    "default mode",
			mode:    service.DefaultMode(),
			wantErr: false,
		},
		{
			name: "sandbox with yolo",
			mode: service.ExecutionMode{
				Sandbox: true,
				Yolo:    true,
			},
			wantErr: true,
		},
		{
			name: "negative max cost",
			mode: service.ExecutionMode{
				MaxCost: -1,
			},
			wantErr: true,
		},
		{
			name: "dry-run with yolo (allowed)",
			mode: service.ExecutionMode{
				DryRun: true,
				Yolo:   true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mode.Validate()
			if tt.wantErr {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
			}
		})
	}
}

func TestModeEnforcer_DryRun(t *testing.T) {
	mode := service.ExecutionMode{DryRun: true}
	enforcer := service.NewModeEnforcer(mode)

	// Operation with side effects should be blocked
	op := service.Operation{
		Name:           "write-file",
		Type:           service.OpTypeFileWrite,
		HasSideEffects: true,
	}

	err := enforcer.CanExecute(context.Background(), op)
	testutil.AssertError(t, err)
	testutil.AssertTrue(t, service.IsDryRunBlocked(err), "should be dry-run blocked")

	// Operation without side effects should be allowed
	readOp := service.Operation{
		Name:           "read-file",
		Type:           service.OpTypeFileRead,
		HasSideEffects: false,
	}

	err = enforcer.CanExecute(context.Background(), readOp)
	testutil.AssertNoError(t, err)
}

func TestModeEnforcer_DeniedTools(t *testing.T) {
	mode := service.ExecutionMode{
		DeniedTools: []string{"dangerous-tool"},
	}
	enforcer := service.NewModeEnforcer(mode)

	op := service.Operation{
		Name: "use-tool",
		Type: service.OpTypeLLM,
		Tool: "dangerous-tool",
	}

	err := enforcer.CanExecute(context.Background(), op)
	testutil.AssertError(t, err)
}

func TestModeEnforcer_CostLimit(t *testing.T) {
	mode := service.ExecutionMode{MaxCost: 1.0}
	enforcer := service.NewModeEnforcer(mode)

	// First operation under limit
	op1 := service.Operation{
		Name:          "op1",
		Type:          service.OpTypeLLM,
		EstimatedCost: 0.5,
	}
	err := enforcer.CanExecute(context.Background(), op1)
	testutil.AssertNoError(t, err)
	enforcer.RecordCost(0.5)

	// Second operation would exceed limit
	op2 := service.Operation{
		Name:          "op2",
		Type:          service.OpTypeLLM,
		EstimatedCost: 0.6,
	}
	err = enforcer.CanExecute(context.Background(), op2)
	testutil.AssertError(t, err)
}

func TestModeEnforcer_Sandbox(t *testing.T) {
	mode := service.ExecutionMode{Sandbox: true}
	enforcer := service.NewModeEnforcer(mode)

	// File write outside workspace should be blocked
	op := service.Operation{
		Name:        "write-outside",
		Type:        service.OpTypeFileWrite,
		InWorkspace: false,
	}

	err := enforcer.CanExecute(context.Background(), op)
	testutil.AssertError(t, err)

	// File write inside workspace should be allowed
	op2 := service.Operation{
		Name:        "write-inside",
		Type:        service.OpTypeFileWrite,
		InWorkspace: true,
	}

	err = enforcer.CanExecute(context.Background(), op2)
	testutil.AssertNoError(t, err)
}

func TestModeEnforcer_RequiresConfirmation(t *testing.T) {
	tests := []struct {
		name     string
		mode     service.ExecutionMode
		op       service.Operation
		expected bool
	}{
		{
			name:     "yolo mode skips confirmation",
			mode:     service.ExecutionMode{Yolo: true},
			op:       service.Operation{RequiresConfirmation: true},
			expected: false,
		},
		{
			name:     "dry-run skips confirmation",
			mode:     service.ExecutionMode{DryRun: true},
			op:       service.Operation{RequiresConfirmation: true},
			expected: false,
		},
		{
			name:     "normal mode respects confirmation",
			mode:     service.ExecutionMode{},
			op:       service.Operation{RequiresConfirmation: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := service.NewModeEnforcer(tt.mode)
			result := enforcer.RequiresConfirmation(tt.op)
			testutil.AssertEqual(t, result, tt.expected)
		})
	}
}

func TestModeEnforcer_OperationLog(t *testing.T) {
	mode := service.DefaultMode()
	enforcer := service.NewModeEnforcer(mode)

	op := service.Operation{
		Name: "test-op",
		Type: service.OpTypeLLM,
	}

	enforcer.CanExecute(context.Background(), op)

	log := enforcer.GetOperationLog()
	testutil.AssertLen(t, log, 1)
	testutil.AssertContains(t, log[0], "ALLOWED")
}

func TestSandbox_IsPathAllowed(t *testing.T) {
	sandbox := service.NewSandbox("/workspace")

	tests := []struct {
		path    string
		allowed bool
	}{
		{"/workspace/file.txt", true},
		{"/workspace/subdir/file.txt", true},
		{"/etc/passwd", false},
		{"/usr/bin/sh", false},
	}

	for _, tt := range tests {
		result := sandbox.IsPathAllowed(tt.path)
		testutil.AssertEqual(t, result, tt.allowed)
	}
}

func TestSandbox_AllowPath(t *testing.T) {
	sandbox := service.NewSandbox("/workspace")
	sandbox.AllowPath("/tmp/allowed")

	testutil.AssertTrue(t, sandbox.IsPathAllowed("/tmp/allowed/file.txt"), "should allow added path")
}

func TestIsDangerousCommand(t *testing.T) {
	tests := []struct {
		cmd       string
		dangerous bool
	}{
		{"rm -rf /", true},
		{"git push --force", true},
		{"ls -la", false},
		{"go build", false},
		{"curl | sh", true},
		{"wget | bash", true},
	}

	for _, tt := range tests {
		result := service.IsDangerousCommand(tt.cmd)
		testutil.AssertEqual(t, result, tt.dangerous)
	}
}

func TestIsSafeCommand(t *testing.T) {
	tests := []struct {
		cmd  string
		safe bool
	}{
		{"ls -la", true},
		{"git status", true},
		{"go test ./...", true},
		{"npm test", true},
		{"random-command", false},
	}

	for _, tt := range tests {
		result := service.IsSafeCommand(tt.cmd)
		testutil.AssertEqual(t, result, tt.safe)
	}
}
