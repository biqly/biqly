package ai

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/biqly/biqly/internal/i18n"
)

// PromptTemplateStore loads static prompt sections (system_rules, output_format).
type PromptTemplateStore interface {
	Template(ctx context.Context, loc i18n.Locale, name string) string
}

type embedPromptStore struct{}

func (embedPromptStore) Template(_ context.Context, loc i18n.Locale, name string) string {
	return promptTemplateFromEmbed(loc, name)
}

type dbPromptStore struct {
	repo promptTemplateRepo
	mu   sync.RWMutex
	cache map[string]string // key: name+"\x00"+locale
}

type promptTemplateRepo interface {
	CountPromptTemplates(ctx context.Context) (int, error)
	GetPromptTemplate(ctx context.Context, name string, loc i18n.Locale) (string, error)
	UpsertPromptTemplate(ctx context.Context, name string, loc i18n.Locale, content string) error
}

var activePromptStore PromptTemplateStore = embedPromptStore{}

// SetPromptTemplateStore switches template resolution to the database (with
// embedded-file fallback). Call once at app startup after migrations.
func SetPromptTemplateStore(store PromptTemplateStore) {
	activePromptStore = store
}

// SeedPromptTemplatesFromEmbed copies embedded defaults into the DB when the
// table is empty. Safe to call on every startup.
func SeedPromptTemplatesFromEmbed(ctx context.Context, repo promptTemplateRepo) error {
	if repo == nil {
		return nil
	}
	n, err := repo.CountPromptTemplates(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	names := []string{"system_rules", "output_format"}
	locales := []i18n.Locale{i18n.LocaleEN, i18n.LocaleTR}
	for _, loc := range locales {
		for _, name := range names {
			body := promptTemplateFromEmbed(loc, name)
			if body == "" {
				continue
			}
			if err := repo.UpsertPromptTemplate(ctx, name, loc, body); err != nil {
				return fmt.Errorf("seed prompt %s/%s: %w", name, loc, err)
			}
		}
	}
	slog.InfoContext(ctx, "seeded ai_prompt_templates from embedded defaults")
	return nil
}

// NewDBPromptTemplateStore resolves templates from metadata DB with in-memory cache.
func NewDBPromptTemplateStore(repo promptTemplateRepo) PromptTemplateStore {
	return &dbPromptStore{repo: repo, cache: make(map[string]string)}
}

func (s *dbPromptStore) Template(ctx context.Context, loc i18n.Locale, name string) string {
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	key := name + "\x00" + string(loc)
	s.mu.RLock()
	if v, ok := s.cache[key]; ok {
		s.mu.RUnlock()
		return v
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.cache[key]; ok {
		return v
	}
	body, err := s.repo.GetPromptTemplate(ctx, name, loc)
	if err != nil {
		slog.WarnContext(ctx, "prompt template db load failed", "name", name, "locale", loc, "error", err)
		body = ""
	}
	if body == "" && loc != i18n.DefaultLocale {
		body, _ = s.repo.GetPromptTemplate(ctx, name, i18n.DefaultLocale)
	}
	if body == "" {
		body = promptTemplateFromEmbed(loc, name)
		if body == "" {
			body = promptTemplateFromEmbed(i18n.DefaultLocale, name)
		}
	}
	s.cache[key] = body
	return body
}

// PromptLocaleForQuestion picks the prompt bundle: Turkish questions use tr
// templates; otherwise UI locale from context, defaulting to en.
func PromptLocaleForQuestion(question string, uiLocale i18n.Locale) i18n.Locale {
	if DetectQuestionLocale(question) == i18n.LocaleTR {
		return i18n.LocaleTR
	}
	if uiLocale != "" {
		return uiLocale
	}
	return i18n.LocaleEN
}

func promptTemplate(ctx context.Context, loc i18n.Locale, name string) string {
	return activePromptStore.Template(ctx, loc, name)
}

// KnownPromptTemplateNames lists editable static sections stored in ai_prompt_templates.
func KnownPromptTemplateNames() []string {
	return []string{"system_rules", "output_format"}
}

// InvalidatePromptTemplateCache drops cached text after an admin edit.
func InvalidatePromptTemplateCache(name string, loc i18n.Locale) {
	if s, ok := activePromptStore.(*dbPromptStore); ok {
		s.invalidate(name, loc)
	}
}

func (s *dbPromptStore) invalidate(name string, loc i18n.Locale) {
	key := name + "\x00" + string(loc)
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()
}

// RestorePromptTemplateFromEmbed overwrites one template from embedded defaults.
func RestorePromptTemplateFromEmbed(ctx context.Context, repo promptTemplateRepo, name string, loc i18n.Locale) error {
	body := promptTemplateFromEmbed(loc, name)
	if body == "" {
		return fmt.Errorf("no embedded template for %s/%s", name, loc)
	}
	if err := repo.UpsertPromptTemplate(ctx, name, loc, body); err != nil {
		return err
	}
	InvalidatePromptTemplateCache(name, loc)
	return nil
}

// ReseedAllPromptTemplatesFromEmbed replaces DB content with embedded files (admin).
func ReseedAllPromptTemplatesFromEmbed(ctx context.Context, repo promptTemplateRepo) error {
	if deleter, ok := repo.(interface {
		DeleteAllPromptTemplates(context.Context) error
	}); ok {
		if err := deleter.DeleteAllPromptTemplates(ctx); err != nil {
			return err
		}
	}
	if s, ok := activePromptStore.(*dbPromptStore); ok {
		s.mu.Lock()
		s.cache = make(map[string]string)
		s.mu.Unlock()
	}
	return SeedPromptTemplatesFromEmbed(ctx, repo)
}
