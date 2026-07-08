// Package main generates an OpenAPI 3.0 spec from the live chi router.
//
// It boots the monolith's Router with nil DB/handler dependencies (middlewares
// like RequirePermission are pass-through when the auth client is nil), walks
// the resulting route tree via chi.Walk, and writes a minimal OpenAPI 3.0 JSON
// document to stdout or -o <file>.
//
// The spec intentionally carries no request/response schemas — its purpose is
// to feed ZAP's ajax-spiders and active scanners with a complete, accurate
// endpoint list so no route is missed during DAST scanning.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	bihttp "github.com/biqly/biqly/internal/http"
	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
)

func main() {
	output := flag.String("o", "", "output file (default: stdout)")
	flag.Parse()
	slog.SetDefault(slog.New(slog.DiscardHandler))

	spec := generateOpenAPI()

	var buf []byte
	var err error
	if *output != "" {
		buf, err = sonic.MarshalIndent(spec, "", "  ")
	} else {
		buf, err = sonic.Marshal(spec)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal openapi: %v\n", err)
		os.Exit(1)
	}

	if *output != "" {
		// The generated spec contains route metadata only and must be readable by
		// the ZAP Docker user in CI.
		if err := os.WriteFile(*output, append(buf, '\n'), 0644); err != nil { //nolint:gosec
			fmt.Fprintf(os.Stderr, "write %s: %v\n", *output, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "OpenAPI spec written to %s (%d endpoints)\n", *output, countPaths(spec))
	} else {
		_, _ = os.Stdout.Write(buf)
		_, _ = os.Stdout.Write([]byte("\n"))
	}
}

// openapiDoc is a minimal OpenAPI 3.0 document with only the fields ZAP needs.
type openapiDoc struct {
	OpenAPI    string                    `json:"openapi"`
	Info       openapiInfo               `json:"info"`
	Servers    []openapiServer           `json:"servers,omitempty"`
	Paths      map[string]map[string]any `json:"paths"`
	Components openapiComponents         `json:"components,omitempty"`
}

type openapiInfo struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

type openapiServer struct {
	URL string `json:"url"`
}

type openapiComponents struct {
	SecuritySchemes map[string]any `json:"securitySchemes,omitempty"`
}

func generateOpenAPI() *openapiDoc {
	// Build the monolith router with zero external dependencies. Config has
	// no auth service URL → NewAuthClient returns nil → RequirePermission
	// and RequireDatasourceAccess become pass-through, so every route is
	// registered and walkable.
	cfg := &config.Config{}
	deps := &app.Dependencies{Config: cfg}

	routes := bihttp.ChiRouter(deps)
	endpoints := collectEndpoints(routes)
	return buildDoc(endpoints)
}

// endpoint is a single method+path pair discovered via chi.Walk.
type endpoint struct {
	method string
	path   string
}

func collectEndpoints(routes chi.Routes) []endpoint {
	var endpoints []endpoint

	_ = chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		endpoints = append(endpoints, endpoint{method: method, path: route})
		return nil
	})

	// Deduplicate (chi.Walk can report HEAD alongside GET).
	seen := make(map[string]bool, len(endpoints))
	deduped := endpoints[:0]
	for _, ep := range endpoints {
		key := ep.method + " " + ep.path
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, ep)
		}
	}
	endpoints = deduped

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].path != endpoints[j].path {
			return endpoints[i].path < endpoints[j].path
		}
		return endpoints[i].method < endpoints[j].method
	})

	return endpoints
}

func buildDoc(endpoints []endpoint) *openapiDoc {
	paths := make(map[string]map[string]any)

	for _, ep := range endpoints {
		if ep.method == "OPTIONS" || ep.method == "TRACE" || ep.method == "HEAD" {
			continue
		}

		oapiPath := chiToOpenAPIPath(ep.path)
		if paths[oapiPath] == nil {
			paths[oapiPath] = make(map[string]any)
		}

		method := strings.ToLower(ep.method)
		paths[oapiPath][method] = map[string]any{
			"summary":     fmt.Sprintf("%s %s", ep.method, ep.path),
			"description": "Auto-discovered route. Auth required in production.",
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Successful response",
				},
			},
		}
	}

	return &openapiDoc{
		OpenAPI: "3.0.3",
		Info: openapiInfo{
			Title:       "Biqly API",
			Description: "Auto-generated from chi router for DAST scanning (OWASP ZAP). Request/response schemas omitted — this spec is for endpoint discovery only.",
			Version:     "1.0.0",
		},
		Servers: []openapiServer{
			{URL: "https://abi.il1.nl"},
		},
		Paths: paths,
		Components: openapiComponents{
			SecuritySchemes: map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
				"apiKey": map[string]any{
					"type": "apiKey",
					"in":   "header",
					"name": "X-API-Key",
				},
			},
		},
	}
}

// chiToOpenAPIPath normalizes chi route patterns for OpenAPI.
// chi.Walk strips param names (e.g. /{id} becomes /{}), so we replace
// empty braces with a generic {param} to produce valid OpenAPI paths.
// Regex params ({id:\d+}) are simplified to {param} as well.
func chiToOpenAPIPath(path string) string {
	// Collapse repeated {} pairs that chi emits for each route param.
	// Each empty {} becomes {param}, numbered sequentially per path.
	counter := 0
	result := strings.Builder{}
	i := 0
	for i < len(path) {
		if i+1 < len(path) && path[i] == '{' && path[i+1] == '}' {
			counter++
			fmt.Fprintf(&result, "{param%d}", counter)
			i += 2
			continue
		}
		if path[i] == '{' {
			// Skip regex param {name:pattern} -> {name}
			end := strings.IndexByte(path[i:], '}')
			if end < 0 {
				result.WriteByte(path[i])
				i++
				continue
			}
			param := path[i : i+end]
			if colon := strings.IndexByte(param, ':'); colon >= 0 {
				result.WriteString(param[:colon])
				result.WriteByte('}')
			} else {
				result.WriteString(param)
				result.WriteByte('}')
			}
			i += end + 1
			continue
		}
		result.WriteByte(path[i])
		i++
	}
	return result.String()
}

func countPaths(doc *openapiDoc) int {
	n := 0
	for _, methods := range doc.Paths {
		n += len(methods)
	}
	return n
}
