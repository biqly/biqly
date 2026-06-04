package oauth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var allowedAuthHosts = map[string][]string{
	"github": {"github.com"},
	"google": {"accounts.google.com"},
}

func ValidateAuthURL(providerName, authURL string) error {
	u, err := url.Parse(authURL)
	if err != nil {
		return fmt.Errorf("parse oauth auth url: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("oauth auth url must use https")
	}
	hosts, ok := allowedAuthHosts[providerName]
	if !ok {
		return fmt.Errorf("unsupported oauth provider: %s", providerName)
	}
	host := strings.ToLower(u.Hostname())
	for _, allowed := range hosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}
	return fmt.Errorf("oauth auth url host not allowed: %s", host)
}
