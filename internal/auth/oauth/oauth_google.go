package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/biqly/biqly/internal/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleProvider struct {
	oauthProviderBase
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		oauthProviderBase: oauthProviderBase{oauthCfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     google.Endpoint,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.profile",
				"https://www.googleapis.com/auth/userinfo.email",
			},
		}},
	}
}

func (p *GoogleProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*auth.OAuthUserInfo, error) {
	client := p.oauthCfg.Client(ctx, token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch google userinfo: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google API userinfo returned status: %d", resp.StatusCode)
	}

	var rawProfile struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawProfile); err != nil {
		return nil, fmt.Errorf("decode google userinfo: %w", err)
	}

	if rawProfile.Email == "" {
		return nil, errors.New("could not retrieve email address from google account")
	}

	return &auth.OAuthUserInfo{
		Sub:       rawProfile.Sub,
		Email:     rawProfile.Email,
		Name:      rawProfile.Name,
		AvatarURL: rawProfile.Picture,
	}, nil
}
