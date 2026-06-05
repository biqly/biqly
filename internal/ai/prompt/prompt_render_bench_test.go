package prompt

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"text/template"
)

// largeBenchTemplate is sized like a real prompt layout (~5 KB) so
// per-call parse cost is observable. The body has ~30 actions to keep the
// parse step non-trivial; the variant uses different {{.A}}…{{.Z}} keys.
var largeBenchTemplate = func() string {
	var b strings.Builder
	b.WriteString("Header section\n")
	for range 30 {
		b.WriteString("{{if .Section")
		b.WriteString(strings.Repeat("x", 8))
		b.WriteString("}}\n## Block {{.Title}} — {{.Body}}\n{{end}}\n")
	}
	b.WriteString(strings.Repeat("Some filler paragraph text. ", 80))
	return b.String()
}()

// BenchmarkRenderPromptTemplate_Cached measures the steady-state cost when
// the same body is rendered repeatedly (production hot path). Should be
// dominated by template.Execute, not parse.
//
// Run:
//
//	go test -bench=BenchmarkRenderPromptTemplate -benchmem ./internal/ai/...
func BenchmarkRenderPromptTemplate_Cached(b *testing.B) {
	data := map[string]any{"Title": "x", "Body": "y"}
	b.ResetTimer()
	for range b.N {
		_ = renderPromptTemplate(largeBenchTemplate, data)
	}
}

// BenchmarkRenderPromptTemplate_NoCache simulates the old behavior: parse
// the template from scratch on every call. The delta against _Cached shows
// how much CPU the templateCache saves per render.
func BenchmarkRenderPromptTemplate_NoCache(b *testing.B) {
	data := map[string]any{"Title": "x", "Body": "y"}
	b.ResetTimer()
	for range b.N {
		tmpl, err := template.New("prompt").Option("missingkey=zero").Parse(largeBenchTemplate)
		if err != nil {
			b.Fatal(err)
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, data); err != nil {
			b.Fatal(err)
		}
		_ = out.String()
	}
}

// BenchmarkRenderPromptTemplate_CachedParallel covers concurrent renderers
// hitting the same cached template. With sync.Map + sync.Pool buffer the
// per-op cost should stay flat as -cpu scales up.
func BenchmarkRenderPromptTemplate_CachedParallel(b *testing.B) {
	data := map[string]any{"Title": "x", "Body": "y"}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = renderPromptTemplate(largeBenchTemplate, data)
		}
	})
}

// Sanity check (not a benchmark): confirm the cache returns the same parsed
// template on the second call so the benchmark really measures cached work.
func TestRenderPromptTemplateCacheReturnsSameParsed(t *testing.T) {
	first, err := getOrParseTemplate(largeBenchTemplate)
	if err != nil {
		t.Fatal(err)
	}
	second, err := getOrParseTemplate(largeBenchTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("expected cached template pointer to be reused across calls")
	}
}

// keep sync imported in case future benchmarks parallelize differently.
var _ = sync.WaitGroup{}
