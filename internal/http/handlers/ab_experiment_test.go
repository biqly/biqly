package handlers

import (
	"context"
	"database/sql/driver"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/ai/abtest"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

type abExperimentFixture struct {
	router chi.Router
	state  *mockDBState
	now    time.Time
}

func setupABExperimentFixture(t *testing.T) abExperimentFixture {
	t.Helper()
	db, state := setupMockDB(t)
	handler := NewABExperimentHandler(&app.AIDeps{MetaRepo: metadata.NewRepository(db)})
	router := chi.NewRouter()
	router.Post("/api/ai/ab-experiments", handler.Create)
	router.Get("/api/ai/ab-experiments", handler.List)
	router.Get("/api/ai/ab-experiments/{id}", handler.Get)
	router.Put("/api/ai/ab-experiments/{id}", handler.Update)
	router.Put("/api/ai/ab-experiments/{id}/status", handler.UpdateStatus)
	router.Post("/api/ai/ab-experiments/{id}/variants", handler.AddVariant)
	return abExperimentFixture{router: router, state: state, now: time.Now()}
}

func TestABExperimentHandler(t *testing.T) {
	fx := setupABExperimentFixture(t)

	t.Run("Create Experiment", func(t *testing.T) {
		fx.state.queries = []queryMock{
			{
				Pattern: "INSERT INTO ab_experiments",
				Cols:    []string{"id", "created_at", "updated_at"},
				Rows: [][]driver.Value{
					{"exp-123", fx.now, fx.now},
				},
			},
		}

		body := `{"name":"Clarification Test","description":"Comparing system prompts","template_name":"clarification","locale":"en"}`
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/ab-experiments", strings.NewReader(body))
		rec := httptest.NewRecorder()
		fx.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp abtest.Experiment
		err := sonic.ConfigStd.NewDecoder(rec.Body).Decode(&resp)
		assert.Nil(t, err)
		assert.Equal(t, "exp-123", resp.ID)
		assert.Equal(t, "Clarification Test", resp.Name)
	})

	t.Run("List Experiments", func(t *testing.T) {
		fx.state.queries = []queryMock{
			{
				Pattern: "SELECT id, name, description, template_name, locale, status",
				Cols:    []string{"id", "name", "description", "template_name", "locale", "status", "started_at", "ended_at", "created_by", "created_at", "updated_at"},
				Rows: [][]driver.Value{
					{"exp-123", "Clarification Test", "desc", "clarification", "en", "draft", nil, nil, nil, fx.now, fx.now},
				},
			},
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/ab-experiments?status=draft", nil)
		rec := httptest.NewRecorder()
		fx.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp []abtest.Experiment
		err := sonic.ConfigStd.NewDecoder(rec.Body).Decode(&resp)
		assert.Nil(t, err)
		assert.Len(t, resp, 1)
		assert.Equal(t, "exp-123", resp[0].ID)
	})

	t.Run("Get Experiment Detail", func(t *testing.T) {
		fx.state.queries = []queryMock{
			{
				Pattern: "FROM ab_experiments WHERE id = $1",
				Cols:    []string{"id", "name", "description", "template_name", "locale", "status", "started_at", "ended_at", "created_by", "created_at", "updated_at"},
				Rows: [][]driver.Value{
					{"exp-123", "Clarification Test", "desc", "clarification", "en", "draft", nil, nil, nil, fx.now, fx.now},
				},
			},
			{
				Pattern: "FROM ab_variants WHERE experiment_id = $1",
				Cols:    []string{"id", "experiment_id", "name", "template_version", "traffic_pct", "is_control"},
				Rows: [][]driver.Value{
					{"var-control", "exp-123", "control", 1, 50, true},
					{"var-treatment", "exp-123", "treatment", 2, 50, false},
				},
			},
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/ab-experiments/exp-123", nil)
		rec := httptest.NewRecorder()
		fx.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp abExperimentDetailResponse
		err := sonic.ConfigStd.NewDecoder(rec.Body).Decode(&resp)
		assert.Nil(t, err)
		assert.Equal(t, "exp-123", resp.Experiment.ID)
		assert.Len(t, resp.Variants, 2)
		assert.Equal(t, "var-control", resp.Variants[0].ID)
	})

	t.Run("Update Experiment Status to Running Validates splits", func(t *testing.T) {
		fx.state.queries = []queryMock{
			{
				Pattern: "FROM ab_experiments WHERE id = $1",
				Cols:    []string{"id", "name", "description", "template_name", "locale", "status", "started_at", "ended_at", "created_by", "created_at", "updated_at"},
				Rows: [][]driver.Value{
					{"exp-123", "Clarification Test", "desc", "clarification", "en", "draft", nil, nil, nil, fx.now, fx.now},
				},
			},
			{
				Pattern: "FROM ab_variants WHERE experiment_id = $1",
				Cols:    []string{"id", "experiment_id", "name", "template_version", "traffic_pct", "is_control"},
				Rows: [][]driver.Value{
					{"var-control", "exp-123", "control", 1, 50, true},
					{"var-treatment", "exp-123", "treatment", 2, 40, false}, // Traffic total = 90
				},
			},
		}

		body := `{"status":"running"}`
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/ai/ab-experiments/exp-123/status", strings.NewReader(body))
		rec := httptest.NewRecorder()
		fx.router.ServeHTTP(rec, req)

		// Expected 400 Bad Request because traffic sum = 90% (not 100%)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "traffic_pct must sum to 100")
	})
}
