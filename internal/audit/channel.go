package audit

import "context"

// Caller channels recorded on audit events so "who saw what" is attributable
// to the surface that issued the request.
const (
	ChannelUI       = "ui"
	ChannelAPI      = "api"
	ChannelMCP      = "mcp"
	ChannelInternal = "internal"
)

type channelKeyType struct{}

var channelKey channelKeyType

// WithChannel tags the context with the calling channel (ui/api/mcp/internal).
func WithChannel(ctx context.Context, channel string) context.Context {
	return context.WithValue(ctx, channelKey, channel)
}

// ChannelFromContext returns the calling channel, defaulting to "api" when
// the request was not tagged.
func ChannelFromContext(ctx context.Context) string {
	if ch, ok := ctx.Value(channelKey).(string); ok && ch != "" {
		return ch
	}
	return ChannelAPI
}
