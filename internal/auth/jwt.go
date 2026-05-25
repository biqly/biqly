package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	DefaultJWTIssuer   = "biqly-auth"
	DefaultJWTAudience = "biqly-api"
)

type JWTClaims struct {
	Email                 string   `json:"email"`
	Roles                 []string `json:"roles"`
	WorkspaceID           string   `json:"workspace_id,omitempty"`
	AccessibleDatasources []string `json:"accessible_datasources,omitempty"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	accessTTL  time.Duration
	issuer     string
	audience   string
}

func NewJWTManager(privatePath, publicPath string, accessTTL time.Duration) (*JWTManager, error) {
	issuer := os.Getenv("BI_AUTH_JWT_ISSUER")
	if issuer == "" {
		issuer = DefaultJWTIssuer
	}
	audience := os.Getenv("BI_AUTH_JWT_AUDIENCE")
	if audience == "" {
		audience = DefaultJWTAudience
	}

	if privateKeyPEM := os.Getenv("BI_AUTH_JWT_PRIVATE_KEY"); privateKeyPEM != "" {
		privKey, err := parseRSAPrivateKeyFromConfig(privateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse jwt private key env: %w", err)
		}
		return &JWTManager{
			privateKey: privKey,
			publicKey:  &privKey.PublicKey,
			accessTTL:  accessTTL,
			issuer:     issuer,
			audience:   audience,
		}, nil
	}

	if privatePath != "" {
		//nolint:gosec
		privBytes, err := os.ReadFile(privatePath)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		privKey, err := parseRSAPrivateKeyFromConfig(string(privBytes))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		pubKey := &privKey.PublicKey
		if publicPath != "" {
			//nolint:gosec
			pubBytes, err := os.ReadFile(publicPath)
			if err != nil {
				return nil, fmt.Errorf("read public key: %w", err)
			}
			pubKey, err = jwt.ParseRSAPublicKeyFromPEM(pubBytes)
			if err != nil {
				return nil, fmt.Errorf("parse public key: %w", err)
			}
		}

		return &JWTManager{
			privateKey: privKey,
			publicKey:  pubKey,
			accessTTL:  accessTTL,
			issuer:     issuer,
			audience:   audience,
		}, nil
	}

	slog.Warn("JWT key paths not configured or missing; generating in-memory development RSA key pair")
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate developer key: %w", err)
	}

	return &JWTManager{
		privateKey: privKey,
		publicKey:  &privKey.PublicKey,
		accessTTL:  accessTTL,
		issuer:     issuer,
		audience:   audience,
	}, nil
}

func (m *JWTManager) Issuer() string   { return m.issuer }
func (m *JWTManager) Audience() string { return m.audience }

func parseRSAPrivateKeyFromConfig(value string) (*rsa.PrivateKey, error) {
	keyPEM := strings.TrimSpace(value)
	if !strings.HasPrefix(keyPEM, "-----BEGIN") {
		decoded, err := base64.StdEncoding.DecodeString(keyPEM)
		if err != nil {
			return nil, fmt.Errorf("decode base64 pem: %w", err)
		}
		keyPEM = strings.TrimSpace(string(decoded))
	}

	if !strings.HasPrefix(keyPEM, "-----BEGIN") {
		return nil, errors.New("jwt private key is not PEM encoded")
	}

	privKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(keyPEM))
	if err == nil {
		return privKey, nil
	}

	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, errors.New("decode private key pem")
	}
	parsedKey, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
	if parseErr != nil {
		return nil, fmt.Errorf("parse pkcs8 private key: %w", parseErr)
	}
	rsaKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return rsaKey, nil
}

func (m *JWTManager) GenerateToken(userID, email string, roles []string, workspaceID string, datasources []string) (string, error) {
	now := time.Now()
	jti, err := generateJTI()
	if err != nil {
		return "", fmt.Errorf("generate jti: %w", err)
	}
	claims := JWTClaims{
		Email:                 email,
		Roles:                 roles,
		WorkspaceID:           workspaceID,
		AccessibleDatasources: datasources,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID,
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

func (m *JWTManager) ValidateToken(tokenStr string) (*JWTClaims, error) {
	parser := jwt.NewParser(
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}),
	)

	token, err := parser.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.publicKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid claims or token")
	}

	return claims, nil
}

func generateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (m *JWTManager) GetPublicKeyPEM() (string, error) {
	pubASN1, err := x509.MarshalPKIXPublicKey(m.publicKey)
	if err != nil {
		return "", err
	}
	pubBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubASN1,
	})
	return string(pubBytes), nil
}
