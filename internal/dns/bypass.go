package dns

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/miekg/dns"
)

var ErrNoBypassResolver = errors.New("no bypass resolver could be determined")

type BypassConfig struct {
	Resolver      netip.Addr
	Domains       []string
	SearchDomains []string // Search domains from resolv.conf
	Ndots         int      // Ndots value from resolv.conf (important for K8s)
	Timeout       int      // Timeout in seconds from resolv.conf
	Attempts      int      // Number of attempts from resolv.conf
}

func DetectBypassConfig(userDomains []string, userResolver netip.Addr) (*BypassConfig, error) {
	if len(userDomains) == 0 {
		return nil, nil //nolint:nilnil
	}

	config := &BypassConfig{
		Domains:  normalizeDomains(userDomains),
		Resolver: userResolver,
	}

	// If no resolver specified, try to detect from original resolv.conf
	if !config.Resolver.IsValid() {
		// Use miekg/dns built-in resolv.conf parser
		clientConfig, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return nil, fmt.Errorf("parsing resolv.conf: %w", err)
		}

		// Use the first nameserver if available
		if len(clientConfig.Servers) > 0 {
			addr, err := netip.ParseAddr(clientConfig.Servers[0])
			if err == nil {
				config.Resolver = addr
			}
		}

		// Also capture search domains from resolv.conf
		// These are domains that get appended to unqualified names
		if len(clientConfig.Search) > 0 {
			config.SearchDomains = clientConfig.Search
			// Optionally add search domains to bypass list
			// This ensures local domain searches work correctly
			for _, searchDomain := range clientConfig.Search {
				normalized := strings.TrimSpace(strings.ToLower(searchDomain))
				if normalized != "" && !containsDomain(config.Domains, normalized) {
					config.Domains = append(config.Domains, normalized)
				}
			}
		}

		// Capture other important resolv.conf settings
		config.Ndots = clientConfig.Ndots       // Number of dots to trigger absolute lookup
		config.Timeout = clientConfig.Timeout   // Query timeout
		config.Attempts = clientConfig.Attempts // Number of attempts
	}

	if !config.Resolver.IsValid() {
		return nil, ErrNoBypassResolver
	}

	return config, nil
}

func normalizeDomains(domains []string) []string {
	normalized := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = strings.TrimSpace(strings.ToLower(domain))
		if domain != "" {
			// Remove trailing dot if present
			domain = strings.TrimSuffix(domain, ".")
			normalized = append(normalized, domain)
		}
	}
	return normalized
}

func containsDomain(domains []string, domain string) bool {
	for _, d := range domains {
		if d == domain {
			return true
		}
	}
	return false
}
