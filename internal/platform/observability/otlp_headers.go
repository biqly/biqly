package observability

import (
	"net/url"
	"os"
	"strings"
)

// parseOTLPHeaders reads OTEL_EXPORTER_OTLP_HEADERS (key=value pairs, comma-separated).
func parseOTLPHeaders(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	headers := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		if decoded, err := url.QueryUnescape(strings.TrimSpace(value)); err == nil {
			value = decoded
		}
		headers[strings.TrimSpace(key)] = value
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func otlpHeadersFromEnv() map[string]string {
	return parseOTLPHeaders(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))
}
