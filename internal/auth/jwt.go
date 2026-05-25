package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	Email                  string   `json:"email"`
	Roles                  []string `json:"roles"`
	WorkspaceID            string   `json:"workspace_id,omitempty"`
	AccessibleDatasources []string `json:"accessible_datasources,omitempty"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	accessTTL  time.Duration
}

func NewJWTManager(privatePath, publicPath string, accessTTL time.Duration) (*JWTManager, error) {
	if privatePath != "" && publicPath != "" {
		//nolint:gosec
		privBytes, err := os.ReadFile(privatePath)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		//nolint:gosec
		pubBytes, err := os.ReadFile(publicPath)
		if err != nil {
			return nil, fmt.Errorf("read public key: %w", err)
		}

		privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privBytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
		if err != nil {
			return nil, fmt.Errorf("parse public key: %w", err)
		}

		return &JWTManager{
			privateKey: privKey,
			publicKey:  pubKey,
			accessTTL:  accessTTL,
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
	}, nil
}

func (m *JWTManager) GenerateToken(userID, email string, roles []string, workspaceID string, datasources []string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		Email:                  email,
		Roles:                  roles,
		WorkspaceID:            workspaceID,
		AccessibleDatasources: datasources,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(m.privateKey)
}

func (m *JWTManager) ValidateToken(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(t *jwt.Token) (any, error) {
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
