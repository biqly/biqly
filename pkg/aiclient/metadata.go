package aiclient

import (
	"context"
)

// Describe generates AI table/column descriptions via POST /api/ai/metadata/describe.
func (c *Client) Describe(ctx context.Context, req DescribeRequest) (*DescribeResponse, error) {
	var resp DescribeResponse
	if err := c.post(ctx, "/metadata/describe", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Embed refreshes table/column embeddings via POST /api/ai/metadata/embed.
func (c *Client) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	var resp EmbedResponse
	if err := c.post(ctx, "/metadata/embed", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Settings returns non-secret runtime AI configuration via GET /api/ai/settings.
func (c *Client) Settings(ctx context.Context) (*SettingsResponse, error) {
	var resp SettingsResponse
	if err := c.get(ctx, "/settings", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
