package split

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const (
	dnsTimeout        = 3 * time.Second
	wildcardPrefixLen = 2 // Length of "*."
	dnsPort           = "53"
)

var (
	ErrInvalidResolver = errors.New("bypass resolver address is not valid")
	ErrNoBypassDomains = errors.New("no bypass domains specified")
)

type Middleware struct {
	bypassResolver netip.Addr
	bypassDomains  []string
	bypassClient   *dns.Client
	logger         interface {
		Info(s string)
		Error(s string)
		Warn(s string)
	}
}

type Settings struct {
	BypassResolver netip.Addr
	BypassDomains  []string
	Timeout        time.Duration // Optional timeout override
	Logger         interface {
		Info(s string)
		Error(s string)
		Warn(s string)
	}
}

func New(settings Settings) (*Middleware, error) {
	if !settings.BypassResolver.IsValid() {
		return nil, ErrInvalidResolver
	}

	if len(settings.BypassDomains) == 0 {
		return nil, ErrNoBypassDomains
	}

	// Use custom timeout if provided, otherwise use default
	timeout := dnsTimeout
	if settings.Timeout > 0 {
		timeout = settings.Timeout
	}

	return &Middleware{
		bypassResolver: settings.BypassResolver,
		bypassDomains:  settings.BypassDomains,
		bypassClient: &dns.Client{
			Net:     "udp",
			Timeout: timeout,
		},
		logger: settings.Logger,
	}, nil
}

func (m *Middleware) String() string {
	return "split"
}

func (m *Middleware) Stop() (err error) {
	return nil
}

func (m *Middleware) Wrap(next dns.Handler) dns.Handler { //nolint:ireturn
	return dns.HandlerFunc(func(w dns.ResponseWriter, request *dns.Msg) {
		if len(request.Question) == 0 {
			next.ServeDNS(w, request)
			return
		}

		question := request.Question[0]
		domain := question.Name

		if m.shouldBypass(domain) {
			m.handleBypassDNS(w, request)
			return
		}

		next.ServeDNS(w, request)
	})
}

func (m *Middleware) shouldBypass(domain string) bool {
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")

	for _, bypassDomain := range m.bypassDomains {
		// Handle wildcard patterns
		if strings.HasPrefix(bypassDomain, "*.") {
			suffix := bypassDomain[wildcardPrefixLen:]
			// Check if the domain ends with or contains the pattern
			// This handles both direct matches and search-domain-appended queries
			// e.g., *.cluster.local matches both:
			//   - redis.svc.cluster.local
			//   - redis.svc.cluster.local.hsd1.mi.comcast.net
			if strings.HasSuffix(domain, suffix) || strings.Contains(domain, suffix+".") {
				return true
			}
		} else {
			// Check for exact match, subdomain, or embedded match
			// This handles search-domain-appended queries generically
			if domain == bypassDomain || 
			   strings.HasSuffix(domain, "."+bypassDomain) || 
			   strings.Contains(domain, bypassDomain+".") {
				return true
			}
		}
	}

	return false
}

func (m *Middleware) handleBypassDNS(w dns.ResponseWriter, r *dns.Msg) {
	bypassAddr := net.JoinHostPort(m.bypassResolver.String(), dnsPort)

	response, _, err := m.bypassClient.Exchange(r, bypassAddr)
	if err != nil {
		m.logger.Error(fmt.Sprintf("bypass DNS query failed for %s: %v",
			r.Question[0].Name, err))
		dns.HandleFailed(w, r)
		return
	}

	err = w.WriteMsg(response)
	if err != nil {
		m.logger.Error(fmt.Sprintf("failed to write DNS response: %v", err))
	}
}
