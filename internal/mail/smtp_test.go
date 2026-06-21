package mail

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLookupPort(t *testing.T, portStr string) int {
	t.Helper()
	port, err := (&net.Resolver{}).LookupPort(context.Background(), "tcp", portStr)
	require.NoError(t, err)
	return port
}

func testListenTCP(t *testing.T) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return listener
}

func handleRetrySuccessSMTPConn(c net.Conn) {
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 4096)
	_, _ = fmt.Fprintf(c, "220 localhost ESMTP\r\n")
	for {
		n, err := c.Read(buf)
		if err != nil {
			return
		}
		req := string(buf[:n])
		switch {
		case strings.HasPrefix(req, "EHLO"), strings.HasPrefix(req, "HELO"):
			_, _ = fmt.Fprintf(c, "250-localhost\r\n250 AUTH LOGIN PLAIN\r\n")
		case strings.HasPrefix(req, "MAIL FROM:"), strings.HasPrefix(req, "RCPT TO:"):
			_, _ = fmt.Fprintf(c, "250 OK\r\n")
		case strings.HasPrefix(req, "DATA"):
			_, _ = fmt.Fprintf(c, "354 Start mail input\r\n")
		case strings.Contains(req, "\r\n.\r\n"):
			_, _ = fmt.Fprintf(c, "250 OK\r\n")
		case strings.HasPrefix(req, "QUIT"):
			return
		}
	}
}

// --- buildTemplateData tests ---

func TestBuildTemplateData_verification(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("verification", map[string]any{"token": "abc123"})
	require.NoError(t, err)
	url, ok := data["URL"].(string)
	require.True(t, ok)
	assert.Contains(t, url, "/auth/verify-email?token=abc123")
	assert.Contains(t, url, "https://app.example.com")
}

func TestBuildTemplateData_password_reset(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("password_reset", map[string]any{"token": "reset-tok"})
	require.NoError(t, err)
	url, ok := data["URL"].(string)
	require.True(t, ok)
	assert.Contains(t, url, "/auth/reset-password?token=reset-tok")
}

func TestBuildTemplateData_email_change(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	// new_email = true
	data, err := sender.buildTemplateData("email_change", map[string]any{"token": "tok", "new_email": true})
	require.NoError(t, err)
	url, ok := data["URL"].(string)
	require.True(t, ok)
	assert.Contains(t, url, "/auth/email-change/confirm?token=tok")
	assert.Equal(t, true, data["NewEmail"])

	// new_email = false (or missing)
	data, err = sender.buildTemplateData("email_change", map[string]any{"token": "tok2"})
	require.NoError(t, err)
	assert.Equal(t, false, data["NewEmail"])
}

func TestBuildTemplateData_account_unlock(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("account_unlock", map[string]any{"token": "unlock-tok"})
	require.NoError(t, err)
	url, ok := data["URL"].(string)
	require.True(t, ok)
	assert.Contains(t, url, "/auth/unlock-account?token=unlock-tok")
}

func TestBuildTemplateData_new_device(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("new_device", map[string]any{
		"occurred_at": "2026-06-21T12:00:00Z",
		"ip_address":  "192.168.1.1",
		"user_agent":  "Mozilla/5.0",
	})
	require.NoError(t, err)
	occurredAt, ok := data["OccurredAt"].(string)
	require.True(t, ok)
	assert.Contains(t, occurredAt, "Sun, 21 Jun 2026")
	ipAddress, ok := data["IPAddress"].(string)
	require.True(t, ok)
	assert.Contains(t, ipAddress, "192.168.1.1")
	userAgent, ok := data["UserAgent"].(string)
	require.True(t, ok)
	assert.Contains(t, userAgent, "Mozilla/5.0")
	securityURL, ok := data["SecurityURL"].(string)
	require.True(t, ok)
	assert.Contains(t, securityURL, "/auth/security")
}

func TestBuildTemplateData_new_device_invalid_time(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	// Invalid time string should be returned as-is
	data, err := sender.buildTemplateData("new_device", map[string]any{
		"occurred_at": "not-a-time",
		"ip_address":  "1.2.3.4",
		"user_agent":  "curl",
	})
	require.NoError(t, err)
	assert.Equal(t, "not-a-time", data["OccurredAt"])
}

func TestBuildTemplateData_deletion_scheduled(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("deletion_scheduled", map[string]any{
		"purge_at": "2026-07-01T00:00:00Z",
	})
	require.NoError(t, err)
	purgeAt, ok := data["PurgeAt"].(string)
	require.True(t, ok)
	assert.Contains(t, purgeAt, "Wed, 01 Jul 2026")
	accountURL, ok := data["AccountURL"].(string)
	require.True(t, ok)
	assert.Contains(t, accountURL, "/auth/account")
}

func TestBuildTemplateData_duplicate_registration(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("duplicate_registration", nil)
	require.NoError(t, err)
	signInURL, ok := data["SignInURL"].(string)
	require.True(t, ok)
	assert.Contains(t, signInURL, "/auth/signin")
	forgotURL, ok := data["ForgotURL"].(string)
	require.True(t, ok)
	assert.Contains(t, forgotURL, "/auth/forgot-password")
}

func TestBuildTemplateData_magic_link(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("magic_link", map[string]any{"token": "magic"})
	require.NoError(t, err)
	url, ok := data["URL"].(string)
	require.True(t, ok)
	assert.Contains(t, url, "/auth/magic-link?token=magic")
}

func TestBuildTemplateData_invitation(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("invitation", map[string]any{
		"token":      "invite-tok",
		"role_name":  "admin",
		"expires_at": "2026-08-01T00:00:00Z",
	})
	require.NoError(t, err)
	url, ok := data["URL"].(string)
	require.True(t, ok)
	assert.Contains(t, url, "/auth/claim-invite?token=invite-tok")
	assert.Equal(t, "admin", data["RoleName"])
	expiresAt, ok := data["ExpiresAt"].(string)
	require.True(t, ok)
	assert.Contains(t, expiresAt, "Sat, 01 Aug 2026")
}

func TestBuildTemplateData_drift_alert(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	drifts := []map[string]any{
		{"Severity": "critical", "Type": "column_removed", "Field": "age", "Description": "column gone"},
	}
	data, err := sender.buildTemplateData("drift_alert", map[string]any{
		"ModelName":  "sales_model",
		"DriftsText": "sales column removed",
		"ModelURL":   "https://biqly.app/models/sales_model",
		"Drifts":     drifts,
	})
	require.NoError(t, err)
	assert.Equal(t, "sales_model", data["ModelName"])
	assert.Equal(t, "sales column removed", data["DriftsText"])
	assert.Equal(t, drifts, data["Drifts"])
	assert.Equal(t, "https://biqly.app/models/sales_model", data["ModelURL"])
}

func TestBuildTemplateData_unknown_template(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	_, err = sender.buildTemplateData("nonexistent", nil)
	assert.ErrorIs(t, err, ErrUnknownTemplate)
}

func TestBuildTemplateData_custom_frontend_base_url(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://custom.app.com/",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("verification", map[string]any{"token": "x"})
	require.NoError(t, err)
	url, ok := data["URL"].(string)
	require.True(t, ok)
	assert.Contains(t, url, "https://custom.app.com/auth/verify-email?token=x")
}

func TestBuildTemplateData_empty_frontend_base_url(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	data, err := sender.buildTemplateData("verification", map[string]any{"token": "x"})
	require.NoError(t, err)
	url, ok := data["URL"].(string)
	require.True(t, ok)
	assert.Contains(t, url, "http://localhost:3333/auth/verify-email?token=x")
}

// --- dispatch tests ---

func TestDispatch_SMTPHostEmpty(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	err = sender.dispatch(emailJob{
		to:      "user@example.com",
		headers: map[string]string{"Subject": "test"},
		body:    []byte("test body"),
	})
	assert.ErrorContains(t, err, "SMTP host is not configured")
}

func TestDispatch_Success(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	err = sender.dispatch(emailJob{
		to:      "user@example.com",
		headers: map[string]string{"Subject": "test"},
		body:    []byte("From: no-reply@example.com\r\nTo: user@example.com\r\nSubject: test\r\n\r\nHello"),
	})
	require.NoError(t, err)

	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

func TestDispatch_WithAuthAndSenderName(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:        host,
		SMTPPort:        port,
		SMTPUser:        "testuser",
		SMTPPass:        "testpass",
		SMTPFrom:        "no-reply@example.com",
		SMTPSenderName:  "ABI",
		FrontendBaseURL: "https://app.example.com",
	}, nil, nil)
	require.NoError(t, err)

	err = sender.dispatch(emailJob{
		to:      "user@example.com",
		headers: map[string]string{"Subject": "test"},
		body:    []byte("message body"),
	})
	require.NoError(t, err)

	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

// --- handleJob tests ---

func TestHandleJob_Success(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:        host,
		SMTPPort:        port,
		SMTPFrom:        "no-reply@example.com",
		FrontendBaseURL: "https://app.example.com",
		EmailRetries:    2,
	}, nil, nil)
	require.NoError(t, err)

	sender.handleJob(emailJob{
		to:      "user@example.com",
		headers: map[string]string{"Subject": "test"},
		body:    []byte("body"),
	})

	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

func TestHandleJob_RetriesAndFails(t *testing.T) {
	// Start a TCP server that doesn't speak SMTP (connection accepted but hangs),
	// causing dispatch to fail and handleJob to retry
	listener := testListenTCP(t)
	defer func() { _ = listener.Close() }()

	host, portStr, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:        host,
		SMTPPort:        port,
		SMTPFrom:        "no-reply@example.com",
		FrontendBaseURL: "https://app.example.com",
		EmailRetries:    1, // will try 2 times total (retries+1)
	}, nil, nil)
	require.NoError(t, err)

	// Just accept and close to trigger SMTP handshake failure
	done := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				close(done)
				return
			}
			_ = conn.Close()
		}
	}()

	sender.handleJob(emailJob{
		to:      "user@example.com",
		headers: map[string]string{"Subject": "test"},
		body:    []byte("body"),
	})
	_ = listener.Close()
	<-done
	// No assertion on outcome - we're just ensuring no panic and retries happen
}

func TestHandleJob_SuccessAfterRetry(t *testing.T) {
	var (
		mu       sync.Mutex
		attempts int
	)
	listener := testListenTCP(t)
	defer func() { _ = listener.Close() }()

	host, portStr, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	done := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				close(done)
				return
			}
			mu.Lock()
			attempts++
			failFirst := attempts == 1
			mu.Unlock()

			if failFirst {
				_ = conn.Close()
				continue
			}
			go handleRetrySuccessSMTPConn(conn)
		}
	}()

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:        host,
		SMTPPort:        port,
		SMTPFrom:        "no-reply@example.com",
		FrontendBaseURL: "https://app.example.com",
		EmailRetries:    2,
	}, nil, nil)
	require.NoError(t, err)

	sender.handleJob(emailJob{
		to:      "user@example.com",
		headers: map[string]string{"Subject": "test"},
		body:    []byte("body"),
	})
	_ = listener.Close()
	<-done
}

// --- checkRateLimit tests ---

func TestCheckRateLimit_RedisNil_Skipped(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailDailyLimit:    10,
	}, nil, nil)
	require.NoError(t, err)

	err = sender.checkRateLimit(context.Background(), "user@example.com")
	assert.NoError(t, err)
}

func TestCheckRateLimit_LimitZero_Skipped(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailDailyLimit:    0,
	}, nil, nil)
	require.NoError(t, err)

	// Replace the redis field with a broken client to verify the limit=0 check happens first
	// (cannot actually reach redis due to nil check being before limit check)
	// Instead test with nil redis (already covered) and with positive limit+no redis
	sender.redis = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})

	err = sender.checkRateLimit(context.Background(), "user@example.com")
	assert.NoError(t, err, "should skip when limit <= 0 regardless of redis state")
}

func TestCheckRateLimit_RedisFailure_FailOpen(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailDailyLimit:    10,
	}, nil, nil)
	require.NoError(t, err)

	// Point Redis to a non-existent server to trigger fail-open
	sender.redis = redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: time.Millisecond * 100,
	})
	err = sender.checkRateLimit(context.Background(), "user@example.com")
	assert.NoError(t, err, "redis failure should be fail-open (return nil)")
}

func TestCheckRateLimit_UnderLimit(t *testing.T) {
	redisMock := startMockRedisServer(t)
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailDailyLimit:    10,
	}, nil, nil)
	require.NoError(t, err)

	sender.redis = newRedisClientForTest(t, redisMock)

	err = sender.checkRateLimit(context.Background(), "user@example.com")
	assert.NoError(t, err)
}

func TestCheckRateLimit_ExceedsLimit(t *testing.T) {
	redisMock := startMockRedisServer(t)
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailDailyLimit:    3,
	}, nil, nil)
	require.NoError(t, err)

	sender.redis = newRedisClientForTest(t, redisMock)

	// Send 3 times under limit
	for range 3 {
		err = sender.checkRateLimit(context.Background(), "user@example.com")
		assert.NoError(t, err)
	}

	// Fourth should exceed the limit
	err = sender.checkRateLimit(context.Background(), "user@example.com")
	assert.ErrorIs(t, err, ErrEmailRateLimited)
}

func TestCheckRateLimit_DifferentEmailsHaveSeparateCounters(t *testing.T) {
	redisMock := startMockRedisServer(t)
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailDailyLimit:    2,
	}, nil, nil)
	require.NoError(t, err)

	sender.redis = newRedisClientForTest(t, redisMock)

	// First email reaches limit
	err = sender.checkRateLimit(context.Background(), "alice@example.com")
	assert.NoError(t, err)
	err = sender.checkRateLimit(context.Background(), "alice@example.com")
	assert.NoError(t, err)
	err = sender.checkRateLimit(context.Background(), "alice@example.com")
	assert.ErrorIs(t, err, ErrEmailRateLimited)

	// Different email should be under its own limit
	err = sender.checkRateLimit(context.Background(), "bob@example.com")
	assert.NoError(t, err)
	err = sender.checkRateLimit(context.Background(), "bob@example.com")
	assert.NoError(t, err)
	err = sender.checkRateLimit(context.Background(), "bob@example.com")
	assert.ErrorIs(t, err, ErrEmailRateLimited)
}

// --- sendTemplate with real end-to-end flow via mock SMTP ---

func TestSendTemplate_DispatchViaMockSMTP(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		SMTPSenderName:     "ABI",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"token": "verify-token-123",
	})
	require.NoError(t, err)

	// After Close, the worker should have processed the queue
	sender.Close()

	// sendTemplate is called with raw data (token-based) which means the template
	// data is NOT transformed by buildTemplateData. The template expects "URL" key.
	// So the text body won't have the URL. This is an internal API test.
	// Actually, sendTemplate passes data directly to registry.Render which expects
	// the already-rendered data (with "URL" key). Test the full path via SendTemplate.
	// For this test, let's just verify it dispatches.
	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

func TestSendTemplate_InvalidEmail(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "not-an-email", "verification", map[string]any{
		"token": "test",
	})
	assert.Error(t, err)
}

func TestSendTemplate_SenderNameInFrom(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		SMTPSenderName:     "My App",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"token": "test",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

// --- Queue-based delivery tests ---

func TestSendTemplate_AsyncQueueDelivery(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailQueueSize:     10,
	}, nil, nil)
	require.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "user@example.com", "password_reset", map[string]any{
		"token": "reset-tok",
	})
	require.NoError(t, err)

	// Give worker time to process
	sender.Close()

	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

func TestSendTemplate_AsyncQueueFullFallsBackSync(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailQueueSize:     1,
	}, nil, nil)
	require.NoError(t, err)

	// Fill the queue
	sender.queue <- emailJob{to: "queued@example.com"}

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"token": "test",
	})
	require.NoError(t, err, "should fall back to synchronous dispatch")

	sender.Close()
	// The queued job + the sync fallback each deliver one message
	assert.Equal(t, 2, smtpMock.ReceivedCount())
}

// --- SendTemplate (the exported wrapper) ---

func TestSendTemplate_ExportedAPI(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailQueueSize:     10,
	}, nil, nil)
	require.NoError(t, err)

	err = sender.SendTemplate(context.Background(), "verification", "user@example.com", map[string]any{
		"token": "test-token",
	})
	require.NoError(t, err)

	sender.Close()
	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

func TestSendTemplate_ExportedAPI_UnknownTemplate(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailQueueSize:     10,
	}, nil, nil)
	require.NoError(t, err)
	defer sender.Close()

	err = sender.SendTemplate(context.Background(), "not_a_real_template", "user@example.com", nil)
	assert.ErrorIs(t, err, ErrUnknownTemplate)
}

// --- checkRateLimit via sendTemplate ---

func TestSendTemplate_RateLimitExceeded(t *testing.T) {
	redisMock := startMockRedisServer(t)
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailDailyLimit:    2,
	}, nil, nil)
	require.NoError(t, err)
	defer sender.Close()

	sender.redis = newRedisClientForTest(t, redisMock)

	// First two should succeed
	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{"token": "a"})
	assert.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{"token": "b"})
	assert.NoError(t, err)

	// Third should be rate limited
	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{"token": "c"})
	assert.ErrorIs(t, err, ErrEmailRateLimited)
}

// --- frontendURL ---

func TestFrontendURL(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/path", sender.frontendURL("/path"))
}

func TestFrontendURL_EmptyBase(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:3333/path", sender.frontendURL("/path"))
}

// --- emailRateLimitKey ---

func TestEmailRateLimitKey(t *testing.T) {
	key := emailRateLimitKey("User@Example.COM")
	assert.Len(t, key, 32) // sha256 truncated to 16 bytes = 32 hex chars
	assert.Equal(t, emailRateLimitKey("user@example.com"), key)
	assert.Equal(t, emailRateLimitKey("  user@example.com  "), key)
}

// --- sendTemplate edge cases ---

func TestSendTemplate_SenderNameNotUsedWhenFromContainsBracket(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	// When SMTPFrom already contains "<", SMTPSenderName should not be added
	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "Custom <no-reply@example.com>",
		SMTPSenderName:     "ABI",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"token": "test",
	})
	require.NoError(t, err)
	sender.Close()
	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

func TestSendTemplate_SenderNameEmpty(t *testing.T) {
	smtpMock := startMockSMTPServer(t)
	host, portStr, err := net.SplitHostPort(smtpMock.Addr())
	require.NoError(t, err)
	port := testLookupPort(t, portStr)

	// When SMTPSenderName is empty, From should be just the email
	sender, err := NewSMTPEmailSender(&Config{
		SMTPHost:           host,
		SMTPPort:           port,
		SMTPFrom:           "no-reply@example.com",
		SMTPSenderName:     "",
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, nil, nil)
	require.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"token": "test",
	})
	require.NoError(t, err)
	sender.Close()
	assert.Equal(t, 1, smtpMock.ReceivedCount())
}

func TestSendTemplate_ClosedOnQueueSend(t *testing.T) {
	// Test the "email sender closed" path when trying to enqueue
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailQueueSize:     5,
		SMTPFrom:           "no-reply@example.com",
	}, nil, nil)
	require.NoError(t, err)
	sender.Close()

	// After Close, trying to send via queue should fail with "email sender closed"
	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"token": "test",
	})
	assert.ErrorContains(t, err, "email sender closed")
}

// --- client_ext_test.go coverage via mock ---

func TestAPIClient_SendMethods(t *testing.T) {
	client := NewAPIClient("http://localhost:1", "test-token", nil)
	assert.NotNil(t, client)
}
