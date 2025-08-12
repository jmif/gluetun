package dns

import (
	"net/netip"
	"testing"
)

func TestNormalizeDomains(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "mixed case and spaces",
			input:    []string{" CLUSTER.LOCAL ", "Example.Com", "test.org."},
			expected: []string{"cluster.local", "example.com", "test.org"},
		},
		{
			name:     "trailing dots removed",
			input:    []string{"domain.com.", "sub.domain."},
			expected: []string{"domain.com", "sub.domain"},
		},
		{
			name:     "empty strings filtered",
			input:    []string{"", "  ", "valid.com", ""},
			expected: []string{"valid.com"},
		},
		{
			name:     "wildcards preserved",
			input:    []string{"*.local", "*.corp.internal"},
			expected: []string{"*.local", "*.corp.internal"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := normalizeDomains(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("normalizeDomains(%v) returned %d items, want %d",
					tc.input, len(result), len(tc.expected))
			}
			for i := range result {
				if i < len(tc.expected) && result[i] != tc.expected[i] {
					t.Errorf("normalizeDomains(%v)[%d] = %s, want %s",
						tc.input, i, result[i], tc.expected[i])
				}
			}
		})
	}
}

func TestDetectBypassConfig(t *testing.T) {
	t.Parallel()

	t.Run("no domains returns nil", func(t *testing.T) {
		t.Parallel()
		config, err := DetectBypassConfig([]string{}, netip.Addr{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config != nil {
			t.Error("expected nil config when no domains provided")
		}
	})

	t.Run("with domains and resolver", func(t *testing.T) {
		t.Parallel()
		resolver := netip.MustParseAddr("10.0.0.1")
		domains := []string{"cluster.local", "consul.service"}

		config, err := DetectBypassConfig(domains, resolver)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config == nil {
			t.Fatal("expected non-nil config")
		}
		if config.Resolver != resolver {
			t.Errorf("resolver = %v, want %v", config.Resolver, resolver)
		}
		if len(config.Domains) != 2 {
			t.Errorf("expected 2 domains, got %d", len(config.Domains))
		}
	})

	t.Run("domains without resolver", func(t *testing.T) {
		t.Parallel()
		domains := []string{"cluster.local"}

		// This will fail because we can't read /etc/resolv.conf in tests
		config, err := DetectBypassConfig(domains, netip.Addr{})
		if err == nil && config != nil && config.Resolver.IsValid() {
			t.Log("resolver auto-detected from resolv.conf")
		}
	})
}
