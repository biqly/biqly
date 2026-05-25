package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"
)

type OAuthUserInfo struct {
	Sub       string
	Email     string
	Name      string
	AvatarURL string
}

type OAuthProvider interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error)
}

func NewOAuthProvider(name string, cfg *Config) (OAuthProvider, error) {
	switch name {
	case "github":
		if cfg.GitHubClientID == "" || cfg.GitHubClientSecret == "" {
			return nil, errors.New("github oauth credentials not configured")
		}
		return NewGitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURL), nil
	case "google":
		if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
			return nil, errors.New("google oauth credentials not configured")
		}
		return NewGoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL), nil
	default:
		return nil, fmt.Errorf("unsupported oauth provider: %s", name)
	}
}
