package oauth

import (
	"context"
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
		if err := requireCredentials(name, cfg.GitHubClientID, cfg.GitHubClientSecret); err != nil {
			return nil, err
		}
		return NewGitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.GitHubRedirectURL), nil
	case "google":
		if err := requireCredentials(name, cfg.GoogleClientID, cfg.GoogleClientSecret); err != nil {
			return nil, err
		}
		return NewGoogleProvider(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL), nil
	default:
		return nil, fmt.Errorf("unsupported oauth provider: %s", name)
	}
}

type oauthProviderBase struct {
	oauthCfg *oauth2.Config
}

func (p oauthProviderBase) GetAuthURL(state string) string {
	return p.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p oauthProviderBase) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.oauthCfg.Exchange(ctx, code)
}

func requireCredentials(name, clientID, clientSecret string) error {
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("%s oauth credentials not configured", name)
	}
	return nil
}
