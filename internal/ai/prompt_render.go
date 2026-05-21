package ai

import (
	"bytes"
	"text/template"
)

func renderPromptTemplate(body string, data any) string {
	tmpl, err := template.New("prompt").Option("missingkey=zero").Parse(body)
	if err != nil {
		return body
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return body
	}
	return out.String()
}
