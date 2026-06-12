package handlers

import (
	"net/http"

	"github.com/biqly/biqly/internal/http/response"
)

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	v, ok := response.DecodeJSON[T](w, r)
	if !ok {
		var zero T
		return zero, false
	}
	return *v, true
}

func decodeJSONAllowEmpty[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	v, ok := response.DecodeJSONAllowEmpty[T](w, r)
	if !ok {
		var zero T
		return zero, false
	}
	return *v, true
}
