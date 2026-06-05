package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/biqly/biqly/internal/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type GitHubProvider struct {
	oauthCfg *oauth2.Config
}

func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		oauthCfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     github.Endpoint,
			Scopes:       []string{"read:user", "user:email"},
		},
	}
}

func (p *GitHubProvider) GetAuthURL(state string) string {
	return p.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.oauthCfg.Exchange(ctx, code)
}

//nolint:gocognit // profile fetch plus optional secondary emails API when primary email is private
func (p *GitHubProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*auth.OAuthUserInfo, error) {
	client := p.oauthCfg.Client(ctx, token)

	// Fetch primary user profile
	reqProfile, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", http.NoBody)
	if err != nil {
		return nil, err
	}
	respProfile, err := client.Do(reqProfile)
	if err != nil {
		return nil, fmt.Errorf("fetch github user profile: %w", err)
	}
	defer func() {
		if closeErr := respProfile.Body.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	if respProfile.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API profile returned status: %d", respProfile.StatusCode)
	}

	var rawProfile struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.NewDecoder(respProfile.Body).Decode(&rawProfile); err != nil {
		return nil, fmt.Errorf("decode github user profile: %w", err)
	}

	email := rawProfile.Email

	// If email is empty (private email setting), fetch user emails list
	if email == "" { //nolint:nestif // secondary emails lookup only when profile payload omits email
		reqEmails, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", http.NoBody)
		if err != nil {
			return nil, err
		}
		respEmails, err := client.Do(reqEmails)
		if err != nil {
			return nil, fmt.Errorf("fetch github emails: %w", err)
		}
		defer func() {
			if closeErr := respEmails.Body.Close(); closeErr != nil {
				_ = closeErr
			}
		}()

		if respEmails.StatusCode == http.StatusOK {
			var rawEmails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			if err := json.NewDecoder(respEmails.Body).Decode(&rawEmails); err == nil {
				for _, e := range rawEmails {
					if e.Primary && e.Verified {
						email = e.Email
						break
					}
				}
				if email == "" && len(rawEmails) > 0 {
					email = rawEmails[0].Email
				}
			}
		}
	}

	if email == "" {
		return nil, errors.New("could not retrieve any email address from github account")
	}

	name := rawProfile.Name
	if name == "" {
		name = rawProfile.Login
	}

	return &auth.OAuthUserInfo{
		Sub:       strconv.FormatInt(rawProfile.ID, 10),
		Email:     email,
		Name:      name,
		AvatarURL: rawProfile.AvatarURL,
	}, nil
}
