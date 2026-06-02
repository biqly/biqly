package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/semantic"
)

const ambiguityAnalysisCacheTTL = 5 * time.Minute

type ambiguityAnalysisCacheEntry struct {
	result    ambiguitypkg.AmbiguityResult
	source    string
	expiresAt time.Time
}

func ambiguityAnalysisCacheKey(question string, model *semantic.SemanticModel, glossary []promptpkg.GlossaryEntry, confidenceThreshold float64, llmEnabled bool) string {
	payload, _ := json.Marshal(struct {
		Question            string                    `json:"question"`
		Model               *semantic.SemanticModel   `json:"model"`
		Glossary            []promptpkg.GlossaryEntry `json:"glossary"`
		ConfidenceThreshold float64                   `json:"confidence_threshold"`
		LLMEnabled          bool                      `json:"llm_enabled"`
	}{
		Question:            question,
		Model:               model,
		Glossary:            glossary,
		ConfidenceThreshold: confidenceThreshold,
		LLMEnabled:          llmEnabled,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (s *Service) getCachedAmbiguityAnalysis(key string) (ambiguitypkg.AmbiguityResult, string, bool) {
	value, ok := s.ambiguityCache.Load(key)
	if !ok {
		return ambiguitypkg.AmbiguityResult{}, "", false
	}
	entry := value.(ambiguityAnalysisCacheEntry)
	if time.Now().After(entry.expiresAt) {
		s.ambiguityCache.Delete(key)
		return ambiguitypkg.AmbiguityResult{}, "", false
	}
	return entry.result, entry.source, true
}

func (s *Service) cacheAmbiguityAnalysis(key string, result ambiguitypkg.AmbiguityResult, source string) {
	s.ambiguityCache.Store(key, ambiguityAnalysisCacheEntry{
		result:    result,
		source:    source,
		expiresAt: time.Now().Add(ambiguityAnalysisCacheTTL),
	})
}
