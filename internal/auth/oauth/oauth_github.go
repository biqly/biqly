package oauth

import (
	"context"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"io"
	"net/http"
	"strconv"

	"github.com/biqly/biqly/internal/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type GitHubProvider struct {
	oauthProviderBase
}

func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		oauthProviderBase: oauthProviderBase{oauthCfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     github.Endpoint,
			Scopes:       []string{"read:user", "user:email"},
		}},
	}
}

func (p *GitHubProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*auth.OAuthUserInfo, error) {
	client := p.oauthCfg.Client(ctx, token)
	profile, err := fetchGitHubUserProfile(ctx, client)
	if err != nil {
		return nil, err
	}

	email := profile.Email
	if email == "" {
		email, err = fetchGitHubPrimaryEmail(ctx, client)
		if err != nil {
			return nil, err
		}
	}
	if email == "" {
		return nil, errors.New("could not retrieve any email address from github account")
	}

	name := profile.Name
	if name == "" {
		name = profile.Login
	}

	return &auth.OAuthUserInfo{
		Sub:           strconv.FormatInt(profile.ID, 10),
		Email:         email,
		Name:          name,
		AvatarURL:     profile.AvatarURL,
		EmailVerified: true,
	}, nil
}

type githubUserProfile struct {
	ID        int64
	Login     string
	Name      string
	AvatarURL string
	Email     string
}

func fetchGitHubUserProfile(ctx context.Context, client *http.Client) (githubUserProfile, error) {
	reqProfile, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", http.NoBody)
	if err != nil {
		return githubUserProfile{}, err
	}
	respProfile, err := client.Do(reqProfile)
	if err != nil {
		return githubUserProfile{}, fmt.Errorf("fetch github user profile: %w", err)
	}
	defer func() {
		if closeErr := respProfile.Body.Close(); closeErr != nil {
			_ = closeErr
		}
	}()
	if respProfile.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(respProfile.Body, 512))
		return githubUserProfile{}, fmt.Errorf("github API profile returned status: %d (request-id=%s, body=%q)",
			respProfile.StatusCode, respProfile.Header.Get("X-GitHub-Request-Id"), body)
	}

	var raw struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := sonic.ConfigStd.NewDecoder(respProfile.Body).Decode(&raw); err != nil {
		return githubUserProfile{}, fmt.Errorf("decode github user profile: %w", err)
	}
	return githubUserProfile{
		ID: raw.ID, Login: raw.Login, Name: raw.Name, AvatarURL: raw.AvatarURL, Email: raw.Email,
	}, nil
}

func fetchGitHubPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	reqEmails, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", http.NoBody)
	if err != nil {
		return "", err
	}
	respEmails, err := client.Do(reqEmails)
	if err != nil {
		return "", fmt.Errorf("fetch github emails: %w", err)
	}
	defer func() {
		if closeErr := respEmails.Body.Close(); closeErr != nil {
			_ = closeErr
		}
	}()
	if respEmails.StatusCode != http.StatusOK {
		return "", nil
	}

	var rawEmails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := sonic.ConfigStd.NewDecoder(respEmails.Body).Decode(&rawEmails); err != nil {
		return "", nil
	}
	for _, e := range rawEmails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", nil
}
