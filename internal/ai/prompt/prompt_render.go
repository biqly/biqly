package prompt

import (
	"bytes"
	"fmt"
	"sync"
	"text/template"
)

// templateCache memoizes parsed text/templates keyed by their source body.
// Prompt bodies are large (~10–20 KB) and the same handful of layouts are
// rendered on every NL→query request — without caching we re-parse the
// template per request, which dominates prompt-build CPU under load.
var templateCache sync.Map // map[string]*template.Template

// renderBufPool holds the *bytes.Buffer used to hold a rendered template.
// Each call resets the buffer before use and returns it to the pool, so the
// hot path avoids re-allocating ~16 KB on every prompt.
var renderBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func renderPromptTemplate(body string, data any) string {
	tmpl, err := getOrParseTemplate(body)
	if err != nil {
		return body
	}

	buf, ok := renderBufPool.Get().(*bytes.Buffer)
	if !ok {
		buf = new(bytes.Buffer)
	}
	buf.Reset()
	defer renderBufPool.Put(buf)

	if err := tmpl.Execute(buf, data); err != nil {
		return body
	}
	return buf.String()
}

func getOrParseTemplate(body string) (*template.Template, error) {
	if cached, ok := templateCache.Load(body); ok {
		tmpl, ok := cached.(*template.Template)
		if !ok {
			return nil, fmt.Errorf("prompt template cache: unexpected type %T", cached)
		}
		return tmpl, nil
	}
	tmpl, err := template.New("prompt").Option("missingkey=zero").Parse(body)
	if err != nil {
		return nil, err
	}
	// Multiple goroutines may race here; first store wins, others get the
	// already-stored value back. Parsing is idempotent so this is safe.
	actual, _ := templateCache.LoadOrStore(body, tmpl)
	parsed, ok := actual.(*template.Template)
	if !ok {
		return nil, fmt.Errorf("prompt template cache: unexpected type %T", actual)
	}
	return parsed, nil
}
