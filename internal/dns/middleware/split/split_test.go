package split

import (
	"net"
	"net/netip"
	"testing"

	"github.com/miekg/dns"
)

type mockLogger struct {
	infos  []string
	errors []string
	warns  []string
}

func (m *mockLogger) Info(s string) {
	m.infos = append(m.infos, s)
}

func (m *mockLogger) Error(s string) {
	m.errors = append(m.errors, s)
}

func (m *mockLogger) Warn(s string) {
	m.warns = append(m.warns, s)
}

type mockHandler struct {
	called []string
}

func (m *mockHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) > 0 {
		m.called = append(m.called, r.Question[0].Name)
	}
	resp := &dns.Msg{}
	resp.SetReply(r)
	_ = w.WriteMsg(resp)
}

func TestMiddleware_shouldBypass(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		domains  []string
		query    string
		expected bool
	}{
		{
			name:     "exact match",
			domains:  []string{"cluster.local"},
			query:    "cluster.local",
			expected: true,
		},
		{
			name:     "subdomain match",
			domains:  []string{"cluster.local"},
			query:    "myapp.cluster.local",
			expected: true,
		},
		{
			name:     "wildcard match",
			domains:  []string{"*.internal"},
			query:    "app.internal",
			expected: true,
		},
		{
			name:     "wildcard subdomain match",
			domains:  []string{"*.internal"},
			query:    "db.prod.internal",
			expected: true,
		},
		{
			name:     "no match",
			domains:  []string{"cluster.local", "*.internal"},
			query:    "google.com",
			expected: false,
		},
		{
			name:     "trailing dot handled",
			domains:  []string{"cluster.local"},
			query:    "app.cluster.local.",
			expected: true,
		},
		{
			name:     "case insensitive",
			domains:  []string{"cluster.local"},
			query:    "App.CLUSTER.LOCAL",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := &Middleware{
				bypassDomains: tc.domains,
			}

			result := m.shouldBypass(tc.query)
			if result != tc.expected {
				t.Errorf("shouldBypass(%q) = %v, want %v", tc.query, result, tc.expected)
			}
		})
	}
}

func TestMiddleware_Wrap(t *testing.T) {
	t.Parallel()

	logger := &mockLogger{}

	// Note: Can't test actual bypass DNS queries without a real DNS server
	// This test focuses on the routing logic

	m := &Middleware{
		bypassDomains:  []string{"cluster.local", "*.internal", "*.example.local"},
		bypassResolver: netip.MustParseAddr("10.0.0.53"),
		bypassClient:   &dns.Client{}, // Initialize to prevent nil pointer
		logger:         logger,
	}

	testCases := []struct {
		name           string
		query          string
		expectNextCall bool // Whether we expect the next handler to be called
	}{
		{
			name:           "bypass domain - won't call next",
			query:          "myapp.cluster.local.",
			expectNextCall: false, // Will try bypass DNS (will fail in test)
		},
		{
			name:           "wildcard bypass domain - won't call next",
			query:          "db.internal.",
			expectNextCall: false, // Will try bypass DNS (will fail in test)
		},
		{
			name:           "external domain - calls next",
			query:          "google.com.",
			expectNextCall: true,
		},
		{
			name:           "bypass domain with search domain appended",
			query:          "redis.svc.cluster.local.hsd1.mi.comcast.net.",
			expectNextCall: false, // should bypass because cluster.local is in the middle
		},
		{
			name:           "wildcard match with search domain appended",
			query:          "app.example.local.search.domain.com.",
			expectNextCall: false, // should bypass because example.local is in the middle
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			next := &mockHandler{}
			wrapped := m.Wrap(next)

			msg := &dns.Msg{}
			msg.SetQuestion(tc.query, dns.TypeA)

			// Create a mock response writer
			w := &mockResponseWriter{}
			wrapped.ServeDNS(w, msg)

			if tc.expectNextCall {
				if len(next.called) == 0 {
					t.Error("expected next handler to be called")
				}
			} else {
				if len(next.called) != 0 {
					t.Error("expected next handler not to be called")
				}
				// When bypass is attempted, it will fail and log an error
				if len(logger.errors) == 0 {
					t.Error("expected error to be logged for bypass attempt")
				}
			}
		})
	}
}

type mockResponseWriter struct {
	msg *dns.Msg
}

func (m *mockResponseWriter) LocalAddr() net.Addr         { return nil }
func (m *mockResponseWriter) RemoteAddr() net.Addr        { return nil }
func (m *mockResponseWriter) WriteMsg(msg *dns.Msg) error { m.msg = msg; return nil }
func (m *mockResponseWriter) Write([]byte) (int, error)   { return 0, nil }
func (m *mockResponseWriter) Close() error                { return nil }
func (m *mockResponseWriter) TsigStatus() error           { return nil }
func (m *mockResponseWriter) TsigTimersOnly(bool)         {}
func (m *mockResponseWriter) Hijack()                     {}

func TestNew(t *testing.T) {
	t.Parallel()

	logger := &mockLogger{}

	t.Run("valid settings", func(t *testing.T) {
		t.Parallel()

		settings := Settings{
			BypassResolver: netip.MustParseAddr("10.0.0.53"),
			BypassDomains:  []string{"cluster.local"},
			Logger:         logger,
		}

		m, err := New(settings)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m == nil {
			t.Fatal("expected non-nil middleware")
		}
		if len(m.bypassDomains) != 1 {
			t.Errorf("expected 1 bypass domain, got %d", len(m.bypassDomains))
		}
		if m.bypassDomains[0] != "cluster.local" {
			t.Errorf("expected bypass domain 'cluster.local', got %q", m.bypassDomains[0])
		}
	})

	t.Run("invalid resolver", func(t *testing.T) {
		t.Parallel()

		settings := Settings{
			BypassResolver: netip.Addr{}, // Invalid
			BypassDomains:  []string{"cluster.local"},
			Logger:         logger,
		}

		_, err := New(settings)
		if err == nil {
			t.Error("expected error for invalid resolver")
		}
	})

	t.Run("no domains", func(t *testing.T) {
		t.Parallel()

		settings := Settings{
			BypassResolver: netip.MustParseAddr("10.0.0.53"),
			BypassDomains:  []string{}, // Empty
			Logger:         logger,
		}

		_, err := New(settings)
		if err == nil {
			t.Error("expected error for no domains")
		}
	})
}
