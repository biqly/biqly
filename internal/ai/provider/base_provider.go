package provider

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
)

// providerHooks configures endpoint-specific request/response handling for
// baseProvider. Marshal and parse are provider-owned; HTTP transport is shared.
type providerHooks struct {
	path    string
	headers func(httpProvider) map[string]string
	marshal func(prompt string, temperature float64) ([]byte, error)
	parse   func(body []byte) (GenerationResult, error)
}

// baseProvider runs JSON POST requests with retry and delegates auth/body shaping
// to providerHooks (OpenAI-compatible chat, Anthropic messages, etc.).
type baseProvider struct {
	http        httpProvider
	hooks       providerHooks
	model       string
	maxTokens   int
	temperature float64
	logName     string
}

func (p *baseProvider) generate(ctx context.Context, prompt string) (GenerationResult, error) {
	return p.generateAt(ctx, prompt, p.temperature)
}

func (p *baseProvider) generateAt(ctx context.Context, prompt string, temperature float64) (result GenerationResult, err error) {
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.ProviderGenerate")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("ai.model", p.model),
		attribute.Float64("ai.temperature", temperature),
	)

	estPrompt := promptpkg.EstimateTokens(prompt)
	body, err := p.hooks.marshal(prompt, temperature)
	if err != nil {
		return GenerationResult{}, fmt.Errorf("marshal request: %w", err)
	}
	headers := p.hooks.headers(p.http)
	if headers == nil {
		headers = map[string]string{}
	}

	result, err = execHTTPPostRetry(ctx, p.http.client, httpPostSpec{
		URL:     p.http.url(p.hooks.path),
		Headers: headers,
		Body:    body,
	}, func(status int, respBody []byte) (GenerationResult, error, bool) {
		if status == http.StatusOK {
			gen, parseErr := p.hooks.parse(respBody)
			if parseErr != nil {
				return GenerationResult{}, parseErr, false
			}
			return gen, nil, false
		}
		apiErr := fmt.Errorf("API error %d: %s", status, string(respBody))
		return GenerationResult{}, apiErr, isRetriableHTTPStatus(status)
	})
	if err != nil {
		return GenerationResult{}, err
	}
	if result.Usage != nil {
		span.SetAttributes(
			attribute.Int("ai.tokens.prompt", result.Usage.Prompt),
			attribute.Int("ai.tokens.completion", result.Usage.Completion),
		)
	}
	logLLMCompletion(ctx, p.logName, p.model, estPrompt, result)
	return result, nil
}

// embeddingHooks configures the embeddings endpoint for baseEmbedder.
type embeddingHooks struct {
	path    string
	headers func(httpProvider) map[string]string
	marshal func(texts []string) ([]byte, error)
	parse   func(body []byte, count int) ([][]float32, error)
}

// baseEmbedder runs embedding POST requests with the same HTTP/retry stack as baseProvider.
type baseEmbedder struct {
	http  httpProvider
	hooks embeddingHooks
	model string
}

func (e *baseEmbedder) embed(ctx context.Context, texts []string) (out [][]float32, err error) {
	if len(texts) == 0 {
		return nil, nil
	}

	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.Embed")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("ai.embedding.model", e.model),
		attribute.Int("ai.embedding.batch_size", len(texts)),
	)
	body, err := e.hooks.marshal(texts)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}
	headers := e.hooks.headers(e.http)
	if headers == nil {
		headers = map[string]string{}
	}

	respBody, err := execHTTPPostRetryBytes(ctx, e.http.client, httpPostSpec{
		URL:     e.http.url(e.hooks.path),
		Headers: headers,
		Body:    body,
	}, func(status int, respBody []byte) ([]byte, error, bool) {
		if status == http.StatusOK {
			return respBody, nil, false
		}
		apiErr := fmt.Errorf("embedding API error %d: %s", status, string(respBody))
		return nil, apiErr, isRetriableHTTPStatus(status)
	})
	if err != nil {
		return nil, err
	}
	return e.hooks.parse(respBody, len(texts))
}
