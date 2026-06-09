package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/bytedance/sonic"
	"time"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	ambiguityAnalysisCacheTTL        = 5 * time.Minute
	ambiguityAnalysisCacheMaxEntries = 512
)

type ambiguityAnalysisCacheEntry struct {
	result    ambiguitypkg.Result
	source    string
	expiresAt time.Time
}

func ambiguityAnalysisCacheKey(question string, model *semantic.SemanticModel, glossary []promptpkg.GlossaryEntry, confidenceThreshold float64, llmEnabled bool, synonymOnly bool) string {
	payload, err := sonic.ConfigStd.Marshal(struct {
		Question            string                    `json:"question"`
		Model               *semantic.SemanticModel   `json:"model"`
		Glossary            []promptpkg.GlossaryEntry `json:"glossary"`
		ConfidenceThreshold float64                   `json:"confidence_threshold"`
		LLMEnabled          bool                      `json:"llm_enabled"`
		SynonymOnly         bool                      `json:"synonym_only"`
	}{
		Question:            question,
		Model:               model,
		Glossary:            glossary,
		ConfidenceThreshold: confidenceThreshold,
		LLMEnabled:          llmEnabled,
		SynonymOnly:         synonymOnly,
	})
	if err != nil {
		sum := sha256.Sum256([]byte(question))
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (s *Service) getCachedAmbiguityAnalysis(key string) (ambiguitypkg.Result, string, bool) {
	value, ok := s.ambiguityCache.Load(key)
	if !ok {
		return ambiguitypkg.Result{}, "", false
	}
	entry, ok := value.(ambiguityAnalysisCacheEntry)
	if !ok {
		return ambiguitypkg.Result{}, "", false
	}
	if time.Now().After(entry.expiresAt) {
		s.ambiguityCache.Delete(key)
		return ambiguitypkg.Result{}, "", false
	}
	return entry.result, entry.source, true
}

func (s *Service) cacheAmbiguityAnalysis(key string, result ambiguitypkg.Result, source string) {
	now := time.Now()
	if count := s.pruneAmbiguityAnalysisCache(now); count >= ambiguityAnalysisCacheMaxEntries {
		evicted := false
		s.ambiguityCache.Range(func(cacheKey, _ any) bool {
			s.ambiguityCache.Delete(cacheKey)
			evicted = true
			return false
		})
		if !evicted {
			return
		}
	}
	s.ambiguityCache.Store(key, ambiguityAnalysisCacheEntry{
		result:    result,
		source:    source,
		expiresAt: now.Add(ambiguityAnalysisCacheTTL),
	})
}

func (s *Service) pruneAmbiguityAnalysisCache(now time.Time) int {
	count := 0
	s.ambiguityCache.Range(func(key, value any) bool {
		entry, ok := value.(ambiguityAnalysisCacheEntry)
		if !ok || now.After(entry.expiresAt) {
			s.ambiguityCache.Delete(key)
			return true
		}
		count++
		return true
	})
	return count
}
