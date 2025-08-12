package dns

import (
	"bufio"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
)

const minResolverFields = 2

var ErrNoBypassResolver = errors.New("no bypass resolver could be determined")

type BypassConfig struct {
	Resolver netip.Addr
	Domains  []string
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
		resolvConfig, err := parseResolvConfSimple("/etc/resolv.conf")
		if err != nil {
			return nil, fmt.Errorf("detecting bypass resolver: %w", err)
		}
		if len(resolvConfig.Nameservers) > 0 {
			config.Resolver = resolvConfig.Nameservers[0]
		}
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

type resolvConfigSimple struct {
	Nameservers []netip.Addr
}

func parseResolvConfSimple(path string) (*resolvConfigSimple, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	config := &resolvConfigSimple{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < minResolverFields {
			continue
		}

		if fields[0] == "nameserver" {
			addr, err := netip.ParseAddr(fields[1])
			if err == nil {
				config.Nameservers = append(config.Nameservers, addr)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning file: %w", err)
	}

	return config, nil
}
