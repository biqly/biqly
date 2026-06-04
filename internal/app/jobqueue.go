package app

import (
	"errors"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/queue"
)

func NewAIJobQueue(cfg *config.Config) (queue.AIJobPublisher, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}
	if cfg.NATS.URL != "" {
		return queue.ConnectNATS(queue.NATSConfig{
			URL:     cfg.NATS.URL,
			Stream:  cfg.NATS.Stream,
			Subject: cfg.NATS.Subject,
		})
	}
	return queue.NewLocalAIJobQueue(128), nil
}
