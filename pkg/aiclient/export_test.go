package aiclient

import (
	"net/http"

	"github.com/biqly/biqly/pkg/internalapi"
)

// Exported test helpers — these are only compiled in tests.

func NewAPIErrorFromResponseForTest(status int, body internalapi.Error) *APIError {
	return newAPIErrorFromResponse(status, body)
}

func NewClarificationError(resp *QueryResponse) *ClarificationError {
	return newClarificationError(resp)
}

func SentinelForStatusPublic(status int, code string) error {
	return sentinelForStatus(status, code)
}

func DecodeErrorResponsePublic(resp *http.Response) error {
	return decodeErrorResponse(resp)
}
