package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func TestHttpStatusForDomainError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantOK     bool
	}{
		{"validation", core.ErrValidation("BAD_INPUT", "bad"), http.StatusUnprocessableEntity, true},
		{"not found", core.ErrNotFound("item", "x"), http.StatusNotFound, true},
		{"auth", core.ErrAuth("missing token"), http.StatusUnauthorized, true},
		{"rate limit", core.ErrRateLimit("slow down"), http.StatusTooManyRequests, true},
		{"timeout", core.ErrTimeout("timed out"), http.StatusGatewayTimeout, true},
		{"state (default)", core.ErrState("BAD_STATE", "error"), http.StatusInternalServerError, true},
		{"non-domain error", errors.New("plain"), 0, false},
		{"nil error", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ok := httpStatusForDomainError(tt.err)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
		})
	}
}
