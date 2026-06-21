package mail

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// --- Mock SMTP Server ---

type mockSMTPServer struct {
	addr     string
	listener net.Listener
	mu       sync.Mutex
	received [][]byte // captured message bodies
	failAuth bool     // if true, AUTH fails
	closed   atomic.Bool
}

func startMockSMTPServer(t *testing.T) *mockSMTPServer {
	t.Helper()

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	s := &mockSMTPServer{
		listener: listener,
		addr:     listener.Addr().String(),
	}

	go s.serve()

	t.Cleanup(func() {
		s.closed.Store(true)
		_ = s.listener.Close()
	})

	return s
}

func (s *mockSMTPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed.Load() {
				return
			}
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *mockSMTPServer) dispatchSMTPCommand(rw *bufio.ReadWriter, line string, bodyStarted *bool, body *[]byte) bool {
	switch {
	case strings.HasPrefix(line, "EHLO") || strings.HasPrefix(line, "HELO"):
		_, _ = fmt.Fprintf(rw, "250-localhost\r\n250 AUTH LOGIN PLAIN\r\n")
		_ = rw.Flush()

	case strings.HasPrefix(line, "AUTH"):
		if s.failAuth {
			_, _ = fmt.Fprintf(rw, "535 Authentication failed\r\n")
			_ = rw.Flush()
			return false
		}
		if strings.Contains(line, "LOGIN") {
			_, _ = fmt.Fprintf(rw, "334 VXNlcm5hbWU6\r\n")
			_ = rw.Flush()
			if _, err := rw.ReadString('\n'); err != nil {
				return false
			}
			_, _ = fmt.Fprintf(rw, "334 UGFzc3dvcmQ6\r\n")
			_ = rw.Flush()
			if _, err := rw.ReadString('\n'); err != nil {
				return false
			}
		}
		_, _ = fmt.Fprintf(rw, "235 Authentication successful\r\n")
		_ = rw.Flush()

	case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
		_, _ = fmt.Fprintf(rw, "250 OK\r\n")
		_ = rw.Flush()

	case line == "DATA":
		_, _ = fmt.Fprintf(rw, "354 Start mail input; end with <CRLF>.<CRLF>\r\n")
		_ = rw.Flush()
		*bodyStarted = true
		*body = (*body)[:0]

	case line == "QUIT":
		_, _ = fmt.Fprintf(rw, "221 Bye\r\n")
		_ = rw.Flush()
		return false

	case strings.HasPrefix(line, "RSET"):
		_, _ = fmt.Fprintf(rw, "250 OK\r\n")
		_ = rw.Flush()

	default:
		_, _ = fmt.Fprintf(rw, "250 OK\r\n")
		_ = rw.Flush()
	}
	return true
}

func (s *mockSMTPServer) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// Greeting
	_, _ = fmt.Fprintf(rw, "220 localhost ESMTP mock\r\n")
	_ = rw.Flush()

	var bodyStarted bool
	body := make([]byte, 0, 4096)

	for {
		line, err := rw.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if bodyStarted {
			if line == "." {
				bodyStarted = false
				s.mu.Lock()
				cp := make([]byte, len(body))
				copy(cp, body)
				s.received = append(s.received, cp)
				s.mu.Unlock()
				body = body[:0]
				_, _ = fmt.Fprintf(rw, "250 OK: message accepted\r\n")
				_ = rw.Flush()
				continue
			}
			body = append(body, line...)
			body = append(body, '\n')
			continue
		}

		if !s.dispatchSMTPCommand(rw, line, &bodyStarted, &body) {
			return
		}
	}
}

func (s *mockSMTPServer) ReceivedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.received)
}

func (s *mockSMTPServer) ReceivedBodies() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([][]byte, len(s.received))
	copy(result, s.received)
	return result
}

func (s *mockSMTPServer) Addr() string {
	return s.addr
}

// --- Mock Redis Server (minimal RESP) ---

type mockRedisServer struct {
	addr     string
	listener net.Listener
	mu       sync.Mutex
	counters map[string]int64
	incrErr  error // if non-nil, simulate Redis failure
	closed   atomic.Bool
}

func startMockRedisServer(t *testing.T) *mockRedisServer {
	t.Helper()

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	s := &mockRedisServer{
		listener: listener,
		addr:     listener.Addr().String(),
		counters: make(map[string]int64),
	}

	go s.serve()

	t.Cleanup(func() {
		s.closed.Store(true)
		_ = s.listener.Close()
	})

	return s
}

func (s *mockRedisServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed.Load() {
				return
			}
			continue
		}
		go s.handleConn(conn)
	}
}

// readRESP reads a single RESP frame from r.
func readRESP(r *bufio.Reader) (string, []string, error) {
	first, err := r.ReadString('\n')
	if err != nil {
		return "", nil, err
	}
	first = strings.TrimRight(first, "\r\n")

	if first[0] != '*' {
		return "", nil, fmt.Errorf("expected array, got %q", first)
	}
	count, err := strconv.Atoi(first[1:])
	if err != nil {
		return "", nil, err
	}

	var args []string
	for i := 0; i < count; i++ {
		bulk, err := r.ReadString('\n')
		if err != nil {
			return "", nil, err
		}
		bulk = strings.TrimRight(bulk, "\r\n")
		if bulk[0] != '$' {
			return "", nil, fmt.Errorf("expected bulk string, got %q", bulk)
		}
		bulkLen, err := strconv.Atoi(bulk[1:])
		if err != nil {
			return "", nil, err
		}
		data := make([]byte, bulkLen+2) // +2 for \r\n
		if _, err := io.ReadFull(r, data); err != nil {
			return "", nil, err
		}
		args = append(args, string(data[:bulkLen]))
	}
	if len(args) == 0 {
		return "", nil, errors.New("empty command")
	}
	return strings.ToUpper(args[0]), args[1:], nil
}

func writeRESPError(w *bufio.Writer, msg string) {
	_, _ = fmt.Fprintf(w, "-%s\r\n", msg)
	_ = w.Flush()
}

func writeRESPInteger(w *bufio.Writer, n int64) {
	_, _ = fmt.Fprintf(w, ":%d\r\n", n)
	_ = w.Flush()
}

func writeRESPStatus(w *bufio.Writer, s string) {
	_, _ = fmt.Fprintf(w, "+%s\r\n", s)
	_ = w.Flush()
}

func (s *mockRedisServer) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		cmd, args, err := readRESP(r)
		if err != nil {
			return
		}

		s.mu.Lock()
		incrErr := s.incrErr
		s.mu.Unlock()

		switch cmd {
		case "INCR":
			if incrErr != nil {
				writeRESPError(w, incrErr.Error())
				continue
			}
			if len(args) < 1 {
				writeRESPError(w, "wrong number of args for INCR")
				continue
			}
			key := args[0]
			s.mu.Lock()
			s.counters[key]++
			v := s.counters[key]
			s.mu.Unlock()
			writeRESPInteger(w, v)

		case "EXPIRE":
			writeRESPInteger(w, 1)

		case "PING":
			writeRESPStatus(w, "PONG")

		default:
			writeRESPError(w, "unknown command: "+cmd)
		}
	}
}

// newRedisClientForTest creates a go-redis client connected to the mock Redis server.
func newRedisClientForTest(t *testing.T, s *mockRedisServer) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{
		Addr:         s.addr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     1,
	})
	t.Cleanup(func() { _ = client.Close() })

	// Verify connectivity
	if err := client.Ping(t.Context()).Err(); err != nil {
		t.Fatalf("mock redis ping failed: %v", err)
	}
	return client
}
