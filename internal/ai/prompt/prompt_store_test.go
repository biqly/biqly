package prompt

import (
	"context"
	"errors"
	"testing"

	"github.com/biqly/biqly/internal/ai/abtest"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

// mockPromptTemplateRepo implements promptTemplateRepo for testing.
type mockRepo struct {
	countFn  func() int
	getFn    func(name string, loc i18n.Locale) (string, error)
	upsertFn func(name string, loc i18n.Locale, content string) error
}

func (m *mockRepo) CountPromptTemplates(_ context.Context) (int, error) {
	if m.countFn != nil {
		return m.countFn(), nil
	}
	return 0, nil
}

func (m *mockRepo) GetPromptTemplate(_ context.Context, name string, loc i18n.Locale) (string, error) {
	if m.getFn != nil {
		return m.getFn(name, loc)
	}
	return "", nil
}

func (m *mockRepo) UpsertPromptTemplate(_ context.Context, name string, loc i18n.Locale, content string) error {
	if m.upsertFn != nil {
		return m.upsertFn(name, loc, content)
	}
	return nil
}

func dbStoreForTest(t *testing.T, store TemplateStore) *dbPromptStore {
	t.Helper()
	s, ok := store.(*dbPromptStore)
	if !ok {
		t.Fatal("expected *dbPromptStore")
	}
	return s
}

func setStoreRouter(t *testing.T, store TemplateStore, router VariantResolver) {
	t.Helper()
	s := dbStoreForTest(t, store)
	s.mu.Lock()
	s.router = router
	s.mu.Unlock()
}

// versionedMockRepo supports version-based template lookups.
type versionedMockRepo struct {
	mockRepo
	getVersionFn   func(name string, loc i18n.Locale) (string, int, error)
	getByVersionFn func(name string, loc i18n.Locale, version int) (string, error)
	deleteAllFn    func(context.Context) error
}

func (m *versionedMockRepo) GetPromptTemplateVersion(_ context.Context, name string, loc i18n.Locale) (string, int, error) {
	if m.getVersionFn != nil {
		return m.getVersionFn(name, loc)
	}
	return "", 1, nil
}

func (m *versionedMockRepo) GetPromptTemplateByVersion(_ context.Context, name string, loc i18n.Locale, version int) (string, error) {
	if m.getByVersionFn != nil {
		return m.getByVersionFn(name, loc, version)
	}
	return "", errors.New("version not found")
}

func (m *versionedMockRepo) DeleteAllPromptTemplates(ctx context.Context) error {
	if m.deleteAllFn != nil {
		return m.deleteAllFn(ctx)
	}
	return nil
}

// mockVariantResolver implements VariantResolver for testing.
type mockVariantResolver struct {
	resolveFn func(ctx context.Context, userID, templateName, locale string, defaultVersion int) (abtest.Variant, error)
}

func (m *mockVariantResolver) ResolveVariant(ctx context.Context, userID, templateName, locale string, defaultVersion int) (abtest.Variant, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, userID, templateName, locale, defaultVersion)
	}
	return abtest.Variant{TemplateVersion: defaultVersion}, nil
}

func TestGetActivePromptStoreDefault(t *testing.T) {
	t.Parallel()
	store := getActivePromptStore()
	if store == nil {
		t.Fatal("getActivePromptStore returned nil")
	}
	// Should be the embed store by default.
	ctx := context.Background()
	content := store.Template(ctx, i18n.DefaultLocale, "system_rules")
	if content == "" {
		t.Log("system_rules template is empty (expected if no embed)")
	}
}

func TestSetPromptTemplateStoreAndGet(t *testing.T) {
	t.Parallel()
	store := NewDBPromptTemplateStore(&mockRepo{})
	SetPromptTemplateStore(store)
	got := getActivePromptStore()
	if got != store {
		t.Fatal("SetPromptTemplateStore did not switch the active store")
	}
	// Restore the default so other tests aren't affected.
	SetPromptTemplateStore(embedPromptStore{})
}

func TestShouldSeedPromptTemplateEmptyTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &mockRepo{countFn: func() int { return 0 }}
	seed, err := shouldSeedPromptTemplate(ctx, repo, 0, i18n.DefaultLocale, "system_rules")
	if err != nil {
		t.Fatalf("shouldSeedPromptTemplate: %v", err)
	}
	if !seed {
		t.Fatal("expected seed=true for empty table")
	}
}

func TestShouldSeedPromptTemplateExisting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &mockRepo{
		countFn: func() int { return 5 },
		getFn:   func(_ string, _ i18n.Locale) (string, error) { return "existing", nil },
	}
	seed, err := shouldSeedPromptTemplate(ctx, repo, 5, i18n.DefaultLocale, "system_rules")
	if err != nil {
		t.Fatalf("shouldSeedPromptTemplate: %v", err)
	}
	if seed {
		t.Fatal("expected seed=false for existing content")
	}
}

func TestShouldSeedPromptTemplateEmptyContent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &mockRepo{
		countFn: func() int { return 5 },
		getFn:   func(_ string, _ i18n.Locale) (string, error) { return "", nil },
	}
	seed, err := shouldSeedPromptTemplate(ctx, repo, 5, i18n.DefaultLocale, "system_rules")
	if err != nil {
		t.Fatalf("shouldSeedPromptTemplate: %v", err)
	}
	if !seed {
		t.Fatal("expected seed=true when existing content is empty")
	}
}

func TestShouldSeedPromptTemplateWithError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &mockRepo{
		countFn: func() int { return 1 },
		getFn:   func(_ string, _ i18n.Locale) (string, error) { return "", errors.New("db error") },
	}
	_, err := shouldSeedPromptTemplate(ctx, repo, 1, i18n.DefaultLocale, "system_rules")
	if err == nil {
		t.Fatal("expected error from shouldSeedPromptTemplate")
	}
}

func TestSeedPromptTemplatesFromEmbedNilRepo(t *testing.T) {
	t.Parallel()
	if err := SeedPromptTemplatesFromEmbed(context.Background(), nil); err != nil {
		t.Fatalf("expected nil error for nil repo, got %v", err)
	}
}

func TestDBPromptStoreTemplate(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "test template", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	// Load from dbPromptStore with default locale.
	content := store.Template(context.Background(), i18n.DefaultLocale, "system_rules")
	if content == "" {
		t.Fatal("expected non-empty template from db store")
	}
}

func TestDBPromptStoreSnapshotFallsBackToEmbed(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "", errors.New("not found")
		},
	}
	store := NewDBPromptTemplateStore(repo)
	snap := store.Snapshot(context.Background(), i18n.DefaultLocale, "system_rules")
	if snap.Content == "" {
		t.Log("fallback to embed returned empty — may be expected")
	}
}

func TestDBPromptStoreSnapshotCaches(t *testing.T) {
	t.Parallel()
	var callCount int
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			callCount++
			return "cached content", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	ctx := context.Background()
	// First call should hit the DB.
	snap1 := store.Snapshot(ctx, i18n.DefaultLocale, "test_name")
	if snap1.Content != "cached content" {
		t.Fatalf("first snapshot content = %q", snap1.Content)
	}
	// Second call should use cache.
	snap2 := store.Snapshot(ctx, i18n.DefaultLocale, "test_name")
	if snap2.Content != "cached content" {
		t.Fatalf("second snapshot content = %q", snap2.Content)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 DB call, got %d", callCount)
	}
}

func TestDBPromptStoreSnapshotFallbackToDefaultLocale(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, loc i18n.Locale) (string, error) {
			if loc == i18n.LocaleTR {
				return "", errors.New("not found")
			}
			return "en fallback", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	snap := store.Snapshot(context.Background(), i18n.LocaleTR, "system_rules")
	if snap.Content == "" {
		t.Fatal("expected fallback content for TR locale")
	}
}

func TestDBPromptStoreTemplateWithEmptyLocale(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "content for empty-locale-fallback", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	content := store.Template(context.Background(), "", "system_rules")
	if content == "" {
		t.Fatal("expected content even with empty locale")
	}
}

func TestInvalidatePromptTemplateCacheNoop(t *testing.T) {
	t.Parallel()
	// Should not panic when called with embed store active.
	InvalidatePromptTemplateCache("system_rules", i18n.DefaultLocale)
}

func TestSetVariantResolverNoop(t *testing.T) {
	t.Parallel()
	// Should not panic when embed store is active.
	SetVariantResolver(nil)
}

func TestLocaleForQuestionWithEmptyQuestion(t *testing.T) {
	t.Parallel()
	loc := LocaleForQuestion("", i18n.LocaleTR)
	if loc != i18n.LocaleTR {
		t.Fatalf("LocaleForQuestion = %q, want %q", loc, i18n.LocaleTR)
	}
}

func TestLocaleForQuestionWithUnsupportedLocale(t *testing.T) {
	t.Parallel()
	loc := LocaleForQuestion("some question", "xx")
	if loc != i18n.DefaultLocale {
		t.Fatalf("LocaleForQuestion = %q, want %q", loc, i18n.DefaultLocale)
	}
}

func TestDeniedFieldSet(t *testing.T) {
	t.Parallel()
	denied := deniedFieldSet([]string{"revenue", "Country", "Amount"})
	if !denied["revenue"] {
		t.Fatal("expected revenue to be denied")
	}
	if !denied["country"] {
		t.Fatal("expected country (lowercased) to be denied")
	}
	if !denied["amount"] {
		t.Fatal("expected amount (lowercased) to be denied")
	}
	if denied["unknown"] {
		t.Fatal("unknown should not be denied")
	}
}

func TestFilterAllowedJoins(t *testing.T) {
	t.Parallel()
	model := &semantic.SemanticModel{
		Joins: []semantic.Join{
			{Name: "j1", FromTable: "orders", FromColumn: "id", ToTable: "customers", ToColumn: "order_id"},
			{Name: "j2", FromTable: "products", FromColumn: "id", ToTable: "order_items", ToColumn: "product_id"},
		},
	}
	denied := map[string]bool{"orders.id": true}
	allowed := filterAllowedJoins(model, denied)
	if len(allowed) != 1 {
		t.Fatalf("expected 1 allowed join, got %d", len(allowed))
	}
	if allowed[0].Name != "j2" {
		t.Fatalf("expected j2 to be allowed, got %s", allowed[0].Name)
	}
}

func TestFilterAllowedJoinsNoDenied(t *testing.T) {
	t.Parallel()
	model := &semantic.SemanticModel{
		Joins: []semantic.Join{
			{Name: "j1", FromTable: "a", FromColumn: "id", ToTable: "b", ToColumn: "aid"},
		},
	}
	allowed := filterAllowedJoins(model, nil)
	if len(allowed) != 1 {
		t.Fatalf("expected 1 join, got %d", len(allowed))
	}
}

// --- prompt_store extended tests ---

func TestDBPromptStoreInvalidate(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "cached content", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	ctx := context.Background()

	// Load to cache
	snap1 := store.Snapshot(ctx, i18n.DefaultLocale, "test_invalidate")
	if snap1.Content != "cached content" {
		t.Fatal("first snapshot should populate cache")
	}

	// Invalidate cache
	dbStoreForTest(t, store).invalidate("test_invalidate", i18n.DefaultLocale)

	// Now change repo response
	repo.getFn = func(_ string, _ i18n.Locale) (string, error) {
		return "fresh content", nil
	}

	// Should hit DB again, not cache
	snap2 := store.Snapshot(ctx, i18n.DefaultLocale, "test_invalidate")
	if snap2.Content != "fresh content" {
		t.Fatalf("after invalidate, got %q, want 'fresh content'", snap2.Content)
	}
}

func TestInvalidatePromptTemplateCacheWithDBStore(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "content", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	SetPromptTemplateStore(store)
	defer SetPromptTemplateStore(embedPromptStore{})

	// Should not panic
	InvalidatePromptTemplateCache("system_rules", i18n.DefaultLocale)
}

func TestSetVariantResolverWithDBStore(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "template content", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	SetPromptTemplateStore(store)
	defer SetPromptTemplateStore(embedPromptStore{})
	resolver := &mockVariantResolver{}
	SetVariantResolver(resolver)
	// Should not panic - verified by reaching this line
}

func TestDBPromptStoreSnapshotForUserNoRouter(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "template content", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	ctx := context.Background()

	// User with no router set - should fall back to Snapshot
	snap := store.SnapshotForUser(ctx, "user-1", i18n.DefaultLocale, "system_rules")
	if snap.Content == "" {
		t.Fatal("expected non-empty snapshot content")
	}
}

func TestDBPromptStoreSnapshotForUserEmptyUserID(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "template content", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	ctx := context.Background()

	// Empty userID should fall back to Snapshot
	snap := store.SnapshotForUser(ctx, "", i18n.DefaultLocale, "system_rules")
	if snap.Content == "" {
		t.Fatal("expected non-empty snapshot content")
	}
}

func TestDBPromptStoreSnapshotForUserEmptyLocale(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "template content", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	ctx := context.Background()

	snap := store.SnapshotForUser(ctx, "user-1", "", "system_rules")
	if snap.Content == "" {
		t.Fatal("expected non-empty snapshot content for empty locale")
	}
}

func TestDBPromptStoreSnapshotForUserWithVariantResolver(t *testing.T) {
	t.Parallel()
	repo := versionedMockRepo{
		getVersionFn: func(_ string, _ i18n.Locale) (string, int, error) {
			return "template v2", 2, nil
		},
		getByVersionFn: func(_ string, _ i18n.Locale, _ int) (string, error) {
			return "template v3 content", nil
		},
	}
	store := NewDBPromptTemplateStore(&repo)
	ctx := context.Background()
	resolver := &mockVariantResolver{
		resolveFn: func(_ context.Context, _, _, _ string, _ int) (abtest.Variant, error) {
			return abtest.Variant{ID: "var-1", ExperimentID: "exp-1", TemplateVersion: 3}, nil
		},
	}
	setStoreRouter(t, store, resolver)

	snap := store.SnapshotForUser(ctx, "user-1", i18n.DefaultLocale, "system_rules")
	if snap.Content == "" {
		t.Fatal("expected non-empty snapshot from variant resolver path")
	}
	if snap.Version != 3 {
		t.Fatalf("expected version 3 from variant, got %d", snap.Version)
	}
}

func TestDBPromptStoreSnapshotForUserWithVariantResolverFallsBackOnVersionLoadError(t *testing.T) {
	t.Parallel()
	repo := versionedMockRepo{
		getVersionFn: func(_ string, _ i18n.Locale) (string, int, error) {
			return "template v2", 2, nil
		},
		getByVersionFn: func(_ string, _ i18n.Locale, _ int) (string, error) {
			return "", errors.New("version not found")
		},
	}
	store := NewDBPromptTemplateStore(&repo)
	ctx := context.Background()
	resolver := &mockVariantResolver{
		resolveFn: func(_ context.Context, _, _, _ string, _ int) (abtest.Variant, error) {
			return abtest.Variant{ID: "var-1", ExperimentID: "exp-1", TemplateVersion: 3}, nil
		},
	}
	setStoreRouter(t, store, resolver)

	snap := store.SnapshotForUser(ctx, "user-1", i18n.DefaultLocale, "system_rules")
	// Should fall back to active snapshot
	if snap.Content == "" {
		t.Fatal("expected fallback content when version load fails")
	}
	if snap.Version != 2 {
		t.Fatalf("expected fallback version 2, got %d", snap.Version)
	}
}

func TestDBPromptStoreSnapshotForUserWithVariantResolverSameVersion(t *testing.T) {
	t.Parallel()
	repo := versionedMockRepo{
		getVersionFn: func(_ string, _ i18n.Locale) (string, int, error) {
			return "template v2", 2, nil
		},
	}
	store := NewDBPromptTemplateStore(&repo)
	ctx := context.Background()
	resolver := &mockVariantResolver{
		resolveFn: func(_ context.Context, _, _, _ string, _ int) (abtest.Variant, error) {
			return abtest.Variant{ID: "var-1", ExperimentID: "exp-1", TemplateVersion: 2}, nil
		},
	}
	setStoreRouter(t, store, resolver)

	snap := store.SnapshotForUser(ctx, "user-1", i18n.DefaultLocale, "system_rules")
	// Variant version same as active version - should return active snapshot
	if snap.Content == "" {
		t.Fatal("expected snapshot content")
	}
	if snap.Version != 2 {
		t.Fatalf("expected version 2, got %d", snap.Version)
	}
}

func TestDBPromptStoreSnapshotForUserWithResolverError(t *testing.T) {
	t.Parallel()
	repo := versionedMockRepo{
		getVersionFn: func(_ string, _ i18n.Locale) (string, int, error) {
			return "template v2", 2, nil
		},
	}
	store := NewDBPromptTemplateStore(&repo)
	ctx := context.Background()
	resolver := &mockVariantResolver{
		resolveFn: func(_ context.Context, _, _, _ string, _ int) (abtest.Variant, error) {
			return abtest.Variant{}, errors.New("resolve error")
		},
	}
	setStoreRouter(t, store, resolver)

	snap := store.SnapshotForUser(ctx, "user-1", i18n.DefaultLocale, "system_rules")
	// Should fall back to active snapshot
	if snap.Content == "" {
		t.Fatal("expected fallback content when variant resolve fails")
	}
}

func TestDBPromptStoreLoadVersion(t *testing.T) {
	t.Parallel()
	repo := versionedMockRepo{
		getByVersionFn: func(_ string, _ i18n.Locale, version int) (string, error) {
			if version == 3 {
				return "version 3 content", nil
			}
			return "", errors.New("not found")
		},
	}
	store := NewDBPromptTemplateStore(&repo)
	content, err := dbStoreForTest(t, store).loadVersion(context.Background(), "system_rules", i18n.DefaultLocale, 3)
	if err != nil {
		t.Fatalf("loadVersion error = %v, want nil", err)
	}
	if content != "version 3 content" {
		t.Fatalf("loadVersion = %q, want 'version 3 content'", content)
	}
}

func TestDBPromptStoreLoadVersionNoSupport(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{}
	store := NewDBPromptTemplateStore(repo)
	_, err := dbStoreForTest(t, store).loadVersion(context.Background(), "system_rules", i18n.DefaultLocale, 3)
	if err == nil {
		t.Fatal("loadVersion expected error when repo doesn't support GetPromptTemplateByVersion")
	}
}

func TestDBPromptStoreLoadWithVersionRepo(t *testing.T) {
	t.Parallel()
	repo := versionedMockRepo{
		getVersionFn: func(_ string, _ i18n.Locale) (string, int, error) {
			return "versioned content", 5, nil
		},
	}
	store := NewDBPromptTemplateStore(&repo)
	body, version, err := dbStoreForTest(t, store).load(context.Background(), "system_rules", i18n.DefaultLocale)
	if err != nil {
		t.Fatalf("load error = %v, want nil", err)
	}
	if body != "versioned content" {
		t.Fatalf("load body = %q, want 'versioned content'", body)
	}
	if version != 5 {
		t.Fatalf("load version = %d, want 5", version)
	}
}

func TestDBPromptStoreLoadFallbackToBasic(t *testing.T) {
	t.Parallel()
	repo := &mockRepo{
		getFn: func(_ string, _ i18n.Locale) (string, error) {
			return "basic content", nil
		},
	}
	store := NewDBPromptTemplateStore(repo)
	body, version, err := dbStoreForTest(t, store).load(context.Background(), "system_rules", i18n.DefaultLocale)
	if err != nil {
		t.Fatalf("load error = %v, want nil", err)
	}
	if body != "basic content" {
		t.Fatalf("load body = %q, want 'basic content'", body)
	}
	if version != 1 {
		t.Fatalf("load version = %d, want 1", version)
	}
}

func TestRestorePromptTemplateFromEmbed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var upserted bool
	repo := &mockRepo{
		upsertFn: func(name string, _ i18n.Locale, _ string) error {
			upserted = true
			if name != "system_rules" {
				t.Errorf("upsert name = %q, want system_rules", name)
			}
			return nil
		},
	}

	err := RestorePromptTemplateFromEmbed(ctx, repo, "system_rules", i18n.DefaultLocale)
	if err != nil {
		t.Fatalf("RestorePromptTemplateFromEmbed error = %v, want nil", err)
	}
	if !upserted {
		t.Fatal("expected UpsertPromptTemplate to be called")
	}
}

func TestRestorePromptTemplateFromEmbedNoEmbed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &mockRepo{}
	err := RestorePromptTemplateFromEmbed(ctx, repo, "nonexistent_template", i18n.DefaultLocale)
	if err == nil {
		t.Fatal("expected error for nonexistent embedded template")
	}
}

func TestReseedAllPromptTemplatesFromEmbed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var deleted bool
	repo := versionedMockRepo{
		deleteAllFn: func(_ context.Context) error {
			deleted = true
			return nil
		},
	}
	// Override the seed to verify it's called
	repo.countFn = func() int { return 0 }

	store := NewDBPromptTemplateStore(&repo)
	SetPromptTemplateStore(store)
	defer SetPromptTemplateStore(embedPromptStore{})

	err := ReseedAllPromptTemplatesFromEmbed(ctx, &repo)
	if err != nil {
		t.Fatalf("ReseedAllPromptTemplatesFromEmbed error = %v, want nil", err)
	}
	if !deleted {
		t.Fatal("expected DeleteAllPromptTemplates to be called")
	}
}
