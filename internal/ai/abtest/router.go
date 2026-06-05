package abtest

import (
	"context"
	"sync"
	"time"
)

type cachedExperiment struct {
	Experiment Experiment
	Variants   []Variant
}

type cacheEntry struct {
	experiments []cachedExperiment
	expiresAt   time.Time
}

// TrafficRouter manages variant assignment for active prompt experiments.
type TrafficRouter struct {
	repo     *Repository
	cache    map[string]cacheEntry
	cacheTTL time.Duration
	mu       sync.RWMutex
}

// NewTrafficRouter creates a new TrafficRouter.
func NewTrafficRouter(repo *Repository) *TrafficRouter {
	return &TrafficRouter{
		repo:     repo,
		cache:    make(map[string]cacheEntry),
		cacheTTL: 30 * time.Second,
	}
}

// Invalidate clears the cache for a template and locale.
func (r *TrafficRouter) Invalidate(templateName, locale string) {
	key := templateName + "\x00" + locale
	r.mu.Lock()
	delete(r.cache, key)
	r.mu.Unlock()
}

// ResolveVariant resolves the active experiment variant for a user.
// Returns a fallback variant with defaultVersion if no experiment is running.
func (r *TrafficRouter) ResolveVariant(
	ctx context.Context,
	userID string,
	templateName string,
	locale string,
	defaultVersion int,
) (Variant, error) {
	if templateName == "" {
		return Variant{TemplateVersion: defaultVersion}, nil
	}

	cachedExps, err := r.getRunningExperiments(ctx, templateName, locale)
	if err != nil {
		return Variant{TemplateVersion: defaultVersion}, err
	}

	if len(cachedExps) == 0 {
		return Variant{TemplateVersion: defaultVersion}, nil
	}

	// Pick the first running experiment (sorted DESC by created_at)
	exp := cachedExps[0]
	if len(exp.Variants) == 0 {
		return Variant{TemplateVersion: defaultVersion}, nil
	}

	variant, err := SelectVariantForUser(userID, exp.Experiment.ID, exp.Variants)
	if err != nil {
		return Variant{TemplateVersion: defaultVersion}, err
	}

	return variant, nil
}

func (r *TrafficRouter) getRunningExperiments(ctx context.Context, templateName, locale string) ([]cachedExperiment, error) {
	key := templateName + "\x00" + locale
	r.mu.RLock()
	entry, ok := r.cache[key]
	if ok && time.Now().Before(entry.expiresAt) {
		r.mu.RUnlock()
		return entry.experiments, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok = r.cache[key]
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.experiments, nil
	}

	exps, err := r.repo.GetRunningExperimentsForTemplate(ctx, templateName, locale)
	if err != nil {
		return nil, err
	}

	cachedExps := make([]cachedExperiment, 0, len(exps))
	for _, exp := range exps {
		vars, err := r.repo.ListVariants(ctx, exp.ID)
		if err != nil {
			return nil, err
		}
		cachedExps = append(cachedExps, cachedExperiment{
			Experiment: exp,
			Variants:   vars,
		})
	}

	r.cache[key] = cacheEntry{
		experiments: cachedExps,
		expiresAt:   time.Now().Add(r.cacheTTL),
	}
	return cachedExps, nil
}

// ExperimentTracker captures all resolved variants in a single context request.
type ExperimentTracker struct {
	mu       sync.Mutex
	variants []Variant
}

// AddVariant appends a variant to the tracker thread-safely.
func (t *ExperimentTracker) AddVariant(variant Variant) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.variants = append(t.variants, variant)
}

// GetVariants returns a copy of the tracked variants.
func (t *ExperimentTracker) GetVariants() []Variant {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.variants) == 0 {
		return nil
	}
	out := make([]Variant, len(t.variants))
	copy(out, t.variants)
	return out
}

type trackerContextKey struct{}

// WithExperimentTracker attaches an ExperimentTracker to the context.
func WithExperimentTracker(ctx context.Context, tracker *ExperimentTracker) context.Context {
	return context.WithValue(ctx, trackerContextKey{}, tracker)
}

// TrackerFromContext retrieves the ExperimentTracker from context.
func TrackerFromContext(ctx context.Context) *ExperimentTracker {
	v, _ := ctx.Value(trackerContextKey{}).(*ExperimentTracker)
	return v
}

