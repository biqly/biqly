package oauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/biqly/biqly/internal/auth"
	"golang.org/x/oauth2"
)

type OAuthProvider interface {
	GetAuthURL(state string) string
	ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*auth.OAuthUserInfo, error)
}

func NewOAuthProvider(name string, cfg *auth.Config) (OAuthProvider, error) {
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
