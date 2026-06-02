package prompt

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/biqly/biqly/internal/ai/lingua"
	"github.com/biqly/biqly/internal/i18n"
)

// PromptTemplateSnapshot is the active prompt template content plus its DB version.
type PromptTemplateSnapshot struct {
	Name    string      `json:"name"`
	Locale  i18n.Locale `json:"locale"`
	Content string      `json:"content"`
	Version int         `json:"version"`
}

// PromptTemplateStore loads static prompt sections (system_rules, output_format, retry, clarification).
type PromptTemplateStore interface {
	Template(ctx context.Context, loc i18n.Locale, name string) string
	Snapshot(ctx context.Context, loc i18n.Locale, name string) PromptTemplateSnapshot
}

type embedPromptStore struct{}

func (s embedPromptStore) Template(ctx context.Context, loc i18n.Locale, name string) string {
	return s.Snapshot(ctx, loc, name).Content
}

func (embedPromptStore) Snapshot(_ context.Context, loc i18n.Locale, name string) PromptTemplateSnapshot {
	return PromptTemplateSnapshot{
		Name:    name,
		Locale:  loc,
		Content: promptTemplateFromEmbed(loc, name),
		Version: 1,
	}
}

type dbPromptStore struct {
	repo  promptTemplateRepo
	mu    sync.RWMutex
	cache map[string]PromptTemplateSnapshot // key: name+"\x00"+locale
}

type promptTemplateRepo interface {
	CountPromptTemplates(ctx context.Context) (int, error)
	GetPromptTemplate(ctx context.Context, name string, loc i18n.Locale) (string, error)
	UpsertPromptTemplate(ctx context.Context, name string, loc i18n.Locale, content string) error
}

type promptTemplateVersionRepo interface {
	GetPromptTemplateVersion(ctx context.Context, name string, loc i18n.Locale) (string, int, error)
}

type promptStoreWrapper struct {
	store PromptTemplateStore
}

var activePromptStore atomic.Value

func init() {
	activePromptStore.Store(promptStoreWrapper{store: embedPromptStore{}})
}

func getActivePromptStore() PromptTemplateStore {
	return activePromptStore.Load().(promptStoreWrapper).store
}

// SetPromptTemplateStore switches template resolution to the database (with
// embedded-file fallback). Call once at app startup after migrations.
func SetPromptTemplateStore(store PromptTemplateStore) {
	activePromptStore.Store(promptStoreWrapper{store: store})
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
	for _, loc := range i18n.SupportedLocales {
		for _, name := range KnownPromptTemplateNames() {
			if n > 0 {
				existing, err := repo.GetPromptTemplate(ctx, name, loc)
				if err != nil {
					return err
				}
				if existing != "" {
					// If the existing database template matches the default English fallback,
					// but a locale-specific translation is now available in the embed,
					// we should update it to the new locale-specific translation.
					enFallback := promptTemplateFromEmbed(i18n.DefaultLocale, name)
					locSpecific := promptTemplateFromEmbed(loc, name)
					if loc != i18n.DefaultLocale && existing == enFallback && locSpecific != enFallback {
						// Proceed to seed the locale-specific template
					} else {
						continue
					}
				}
			}
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
	return &dbPromptStore{repo: repo, cache: make(map[string]PromptTemplateSnapshot)}
}

func (s *dbPromptStore) Template(ctx context.Context, loc i18n.Locale, name string) string {
	return s.Snapshot(ctx, loc, name).Content
}

func (s *dbPromptStore) Snapshot(ctx context.Context, loc i18n.Locale, name string) PromptTemplateSnapshot {
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

	body, version, err := s.load(ctx, name, loc)
	if err != nil {
		slog.WarnContext(ctx, "prompt template db load failed", "name", name, "locale", loc, "error", err)
		body = ""
	}
	if body == "" && loc != i18n.DefaultLocale {
		body, version, _ = s.load(ctx, name, i18n.DefaultLocale)
	}
	if body == "" {
		body = promptTemplateFromEmbed(loc, name)
		if body == "" {
			body = promptTemplateFromEmbed(i18n.DefaultLocale, name)
		}
		version = 1
	}
	snap := PromptTemplateSnapshot{Name: name, Locale: loc, Content: body, Version: version}
	s.cache[key] = snap
	return snap
}

func (s *dbPromptStore) load(ctx context.Context, name string, loc i18n.Locale) (string, int, error) {
	if r, ok := s.repo.(promptTemplateVersionRepo); ok {
		return r.GetPromptTemplateVersion(ctx, name, loc)
	}
	body, err := s.repo.GetPromptTemplate(ctx, name, loc)
	if body == "" {
		return body, 0, err
	}
	return body, 1, err
}

// PromptLocaleForQuestion picks the prompt bundle from the detected question locale.
func PromptLocaleForQuestion(question string, uiLocale i18n.Locale) i18n.Locale {
	if strings.TrimSpace(question) != "" {
		if loc, ok := lingua.DetectQuestionLocaleConfident(question); ok {
			return loc
		}
	}
	if i18n.IsSupported(uiLocale) {
		return uiLocale
	}
	return i18n.DefaultLocale
}

func promptTemplate(ctx context.Context, loc i18n.Locale, name string) string {
	return getActivePromptStore().Template(ctx, loc, name)
}

func promptTemplateSnapshot(ctx context.Context, loc i18n.Locale, name string) PromptTemplateSnapshot {
	return getActivePromptStore().Snapshot(ctx, loc, name)
}

// PromptTemplateBundleVersions reports active versions for the editable prompt bundle.
func PromptTemplateBundleVersions(ctx context.Context, loc i18n.Locale) map[string]int {
	out := make(map[string]int, len(KnownPromptTemplateNames()))
	for _, name := range KnownPromptTemplateNames() {
		out[name] = promptTemplateSnapshot(ctx, loc, name).Version
	}
	return out
}

// KnownPromptTemplateNames lists editable static sections stored in ai_prompt_templates.
func KnownPromptTemplateNames() []string {
	return []string{"system_rules", "output_format", "retry", "clarification", "ambiguity", "prompt_layout"}
}

// InvalidatePromptTemplateCache drops cached text after an admin edit.
func InvalidatePromptTemplateCache(name string, loc i18n.Locale) {
	if s, ok := getActivePromptStore().(*dbPromptStore); ok {
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
	if s, ok := getActivePromptStore().(*dbPromptStore); ok {
		s.mu.Lock()
		s.cache = make(map[string]PromptTemplateSnapshot)
		s.mu.Unlock()
	}
	return SeedPromptTemplatesFromEmbed(ctx, repo)
}
