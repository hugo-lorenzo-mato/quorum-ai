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

func TestModeEnforcer_Mode(t *testing.T) {
	mode := service.ExecutionMode{
		DryRun: true,
	}
	enforcer := service.NewModeEnforcer(mode)

	result := enforcer.Mode()
	testutil.AssertTrue(t, result.DryRun, "DryRun should be true")
}

func TestErrDryRunBlocked_Error(t *testing.T) {
	err := service.ErrDryRunBlocked{Operation: "write-file"}
	expected := "operation write-file blocked by dry-run mode"
	testutil.AssertEqual(t, err.Error(), expected)
}
