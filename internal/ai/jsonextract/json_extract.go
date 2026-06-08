package jsonextract

import "strings"

func stripMarkdownJSONFence(s string) string {
	if idx := strings.Index(s, "```json"); idx >= 0 {
		return trimCodeFenceContent(s[idx+len("```json"):])
	}
	if idx := strings.Index(s, "```"); idx >= 0 {
		return trimGenericCodeFenceContent(s[idx+3:])
	}
	return s
}

func trimCodeFenceContent(s string) string {
	if end := strings.Index(s, "```"); end >= 0 {
		s = s[:end]
	}
	return strings.TrimSpace(s)
}

func trimGenericCodeFenceContent(s string) string {
	s = strings.TrimSpace(s)
	if nl := strings.IndexByte(s, '\n'); nl > 0 && nl < 24 {
		first := strings.TrimSpace(s[:nl])
		if strings.EqualFold(first, "json") {
			s = strings.TrimSpace(s[nl+1:])
		}
	}
	return trimCodeFenceContent(s)
}

// stripReasoningPreamble removes an optional "## Reasoning" block emitted before the JSON object.
func stripReasoningPreamble(s string) string {
	const marker = "## Reasoning"
	idx := strings.Index(s, marker)
	if idx < 0 {
		return s
	}
	firstBrace := strings.IndexByte(s, '{')
	if firstBrace >= 0 && firstBrace < idx {
		return s
	}
	rest := strings.TrimSpace(s[idx+len(marker):])
	if brace := strings.IndexByte(rest, '{'); brace >= 0 {
		return strings.TrimSpace(rest[brace:])
	}
	return rest
}

// CleanAIResponseForJSON strips BOM, markdown fences, and leading noise before JSON parsing.
func CleanAIResponseForJSON(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "\ufeff")
	s = stripReasoningPreamble(s)

	s = stripMarkdownJSONFence(s)

	return strings.TrimSpace(s)
}

// ExtractJSONObject returns the first top-level JSON object in s using brace depth and
// string/escape awareness (so `}` inside string values does not end the object early).
func ExtractJSONObject(s string) (obj string, ok bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

// TrimToJSONObject cleans the raw LLM output and isolates a single JSON object if present.
func TrimToJSONObject(raw string) string {
	cleaned := CleanAIResponseForJSON(raw)
	if obj, ok := ExtractJSONObject(cleaned); ok {
		return obj
	}
	return cleaned
}
