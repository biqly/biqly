package observability

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
)

// ClassifyProviderError maps provider/HTTP failures to bounded error_type labels.
func ClassifyProviderError(err error, httpStatus int) string {
	if httpStatus == http.StatusTooManyRequests {
		return "rate_limit"
	}
	if httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden {
		return "auth"
	}
	if isRetriableHTTPStatus(httpStatus) || isRetriableProviderNetErr(err) {
		return "network"
	}
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "unmarshal") || strings.Contains(msg, "parse") || strings.Contains(msg, "json") {
			return "parse"
		}
	}
	return "other"
}

func isRetriableHTTPStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetriableProviderNetErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
		return true
	}
	if opErr, ok := errors.AsType[*net.OpError](err); ok && opErr != nil {
		return true
	}
	return false
}
