// Package mail implements transactional email delivery via SMTP and an internal HTTP API.
package mail

import (
	"crypto/subtle"
	"errors"
	"github.com/bytedance/sonic"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/biqly/biqly/internal/emailaddr"
)

// Server exposes the internal HTTP API the auth service uses to dispatch
// transactional email. It is intended for cluster-internal traffic only and is
// guarded by a shared internal token.
type Server struct {
	sender        *SMTPEmailSender
	internalToken string
}

// NewServer wires the send handler to a configured sender and internal token.
func NewServer(sender *SMTPEmailSender, internalToken string) *Server {
	return &Server{sender: sender, internalToken: internalToken}
}

// Routes returns the chi router for the send API. Health endpoints are mounted
// separately by the worker entrypoint.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/internal/mail/send", s.handleSend)
	return r
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req sendRequest
	if err := sonic.ConfigStd.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Template == "" || req.To == "" {
		http.Error(w, "template and to are required", http.StatusBadRequest)
		return
	}
	if _, err := emailaddr.Normalize(req.To); err != nil {
		http.Error(w, "invalid recipient address", http.StatusBadRequest)
		return
	}

	err := s.sender.SendTemplate(r.Context(), req.Template, req.To, req.Data)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, ErrUnknownTemplate):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrEmailBlocked) || errors.Is(err, ErrEmailRateLimited):
		// Intentional suppression — the message was accepted but not sent.
		slog.Info("mail send suppressed", "template", req.Template, "to", emailaddr.Mask(req.To), "reason", err)
		w.WriteHeader(http.StatusAccepted)
	default:
		slog.Error("mail send failed", "template", req.Template, "to", emailaddr.Mask(req.To), "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (s *Server) authorized(r *http.Request) bool {
	if s.internalToken == "" {
		return false
	}
	provided := r.Header.Get("X-Internal-Token")
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.internalToken)) == 1
}
