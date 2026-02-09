package api

import (
	"errors"
	"net/http"

	"github.com/hugo-lorenzo-mato/quorum-ai/internal/core"
)

func httpStatusForDomainError(err error) (int, bool) {
	var domErr *core.DomainError
	if !errors.As(err, &domErr) || domErr == nil {
		return 0, false
	}

	switch domErr.Category {
	case core.ErrCatValidation:
		return http.StatusUnprocessableEntity, true
	case core.ErrCatNotFound:
		return http.StatusNotFound, true
	case core.ErrCatConflict:
		return http.StatusConflict, true
	case core.ErrCatAuth:
		return http.StatusUnauthorized, true
	case core.ErrCatRateLimit:
		return http.StatusTooManyRequests, true
	case core.ErrCatTimeout:
		return http.StatusGatewayTimeout, true
	default:
		return http.StatusInternalServerError, true
	}
}
