# DNS Bypass

This feature enables selective DNS routing, allowing certain domains to bypass the secure DNS tunnel and use a specified resolver instead.

## Problem

When using Gluetun with DNS-over-TLS:
- Local/internal domains (e.g., *.local, *.cluster.local, *.corp) become unreachable
- Using DNS_KEEP_NAMESERVER=true leaks ALL DNS queries outside the VPN

## Solution

DNS bypass routes queries based on domain patterns:
- Specified domains → Bypass resolver (local/internal DNS)
- All other domains → Secure DNS-over-TLS server

## Configuration

### Basic Usage

```yaml
env:
  # Domains to bypass secure DNS
  - name: DNS_BYPASS_DOMAINS
    value: "cluster.local,consul.service,*.internal.corp"
  
  # DNS server for bypassed domains (optional - auto-detected if not set)
  - name: DNS_BYPASS_RESOLVER
    value: "10.96.0.10"
```

### Examples

#### Kubernetes
```yaml
env:
  - name: DNS_BYPASS_DOMAINS
    value: "cluster.local"
  # Resolver auto-detected from original /etc/resolv.conf
```

#### Docker Swarm
```yaml
env:
  - name: DNS_BYPASS_DOMAINS
    value: "*.swarm,tasks.*"
  - name: DNS_BYPASS_RESOLVER
    value: "127.0.0.11"
```

#### Corporate Network
```yaml
env:
  - name: DNS_BYPASS_DOMAINS
    value: "*.corp.internal,*.ad.company.com"
  - name: DNS_BYPASS_RESOLVER
    value: "10.0.0.53"
```

#### Home Lab
```yaml
env:
  - name: DNS_BYPASS_DOMAINS
    value: "*.home.arpa,*.lan,printer.local"
  - name: DNS_BYPASS_RESOLVER
    value: "192.168.1.1"
```

## How It Works

1. **Configuration**: You specify which domains should bypass secure DNS
2. **Detection**: If no resolver specified, Gluetun reads the original `/etc/resolv.conf` to find the local resolver
3. **Routing**: DNS queries are routed based on domain matching:
   - Domains matching bypass patterns → Bypass resolver
   - All other domains → Secure DoT server

## Wildcard Support

The bypass system supports wildcard patterns:
- `cluster.local` - Matches `cluster.local` and all subdomains
- `*.internal` - Matches any subdomain of `internal` 
- `consul.service` - Matches exactly `consul.service` and its subdomains

## Verification

Check logs for confirmation:
```
INFO DNS bypass configured for domains: [cluster.local consul.service]
INFO DNS bypass enabled for domains: [cluster.local consul.service] using resolver: 10.96.0.10
```

Test bypass domains:
```bash
# Test bypass domain
nslookup myservice.cluster.local
# Should resolve via bypass resolver

# Test external domain
nslookup google.com
# Should resolve through secure DoT
```

## Compatibility

- Works with all VPN providers
- Compatible with any DNS infrastructure (Kubernetes, Docker, corporate, home)
- Replaces the need for `DNS_KEEP_NAMESERVER` in most cases
- Backward compatible with existing configurations

## Migration from DNS_KEEP_NAMESERVER

If currently using `DNS_KEEP_NAMESERVER=true`:

1. Remove `DNS_KEEP_NAMESERVER` environment variable
2. Add `DNS_BYPASS_DOMAINS` with your local domains
3. Optionally specify `DNS_BYPASS_RESOLVER` 
4. External DNS is now secured while local domains still work

## Troubleshooting

### Bypass domains not resolving
- Check logs for "DNS bypass configured" message
- Verify `DNS_BYPASS_DOMAINS` is set correctly
- Confirm bypass resolver is valid

### External DNS not working
- Ensure DoT is enabled and configured
- Check VPN connection status
- Verify DNS_ADDRESS is set to 127.0.0.1

### Auto-detection not working
- Explicitly set `DNS_BYPASS_RESOLVER` to your local DNS server
- Check if `/etc/resolv.conf` exists and is readable