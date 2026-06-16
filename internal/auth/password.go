package auth

import (
	"errors"
	"log"

	"golang.org/x/crypto/bcrypt"
)

const maxBcryptPasswordBytes = 72

var ErrPasswordTooLong = errors.New("password exceeds bcrypt 72-byte limit")

// dummyBcryptHash is a precomputed bcrypt hash used to keep Login response
// times constant when the requested user does not exist. Computed once at
// startup via bcrypt cost=12 over a random secret; the plaintext is discarded.
//
// Do not match the hash to the real password — VerifyPassword(dummyHash, anyInput)
// always returns false. The point is to spend cpu cycles equivalent to a real
// bcrypt verify so timing-based account enumeration becomes infeasible.
var dummyBcryptHash string

func init() {
	// cost=12 hash of an unguessable random string. Generated at package
	// init so the cost matches HashPassword exactly.
	const seed = "biqly-timing-attack-mitigation-seed-do-not-use-as-password"
	h, err := bcrypt.GenerateFromPassword([]byte(seed), 12)
	if err != nil {
		log.Fatalf("auth: init dummy bcrypt hash: %v", err)
	}
	dummyBcryptHash = string(h)
}

func HashPassword(password string) (string, error) {
	if len(password) > maxBcryptPasswordBytes {
		return "", ErrPasswordTooLong
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// VerifyDummyPassword runs a bcrypt comparison against the precomputed dummy
// hash. Used to spend the same wall-clock time as a real password verify when
// the user lookup failed, preventing timing-based account enumeration.
func VerifyDummyPassword(password string) {
	if dummyBcryptHash == "" {
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(dummyBcryptHash), []byte(password)); err != nil {
		_ = err
	}
}
