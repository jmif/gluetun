# DNS Bypass Implementation - COMPLETE

## Problem Statement
When running Gluetun as a Kubernetes sidecar with DNS security enabled, cluster-internal DNS resolution breaks because Gluetun overrides the DNS configuration. Disabling DNS security (DNS_KEEP_NAMESERVER) allows cluster DNS to work but leaks external DNS queries outside the VPN.

## Solution Overview
Implement a split-horizon DNS resolver that:
- Routes Kubernetes internal domains (*.cluster.local) to the cluster DNS server
- Routes all external domains through the secure DNS-over-TLS (DoT) server
- Maintains DNS security for external queries while preserving cluster connectivity

## Stage 1: Kubernetes Environment Detection
**Goal**: Detect Kubernetes environment and preserve cluster DNS configuration
**Success Criteria**: 
- Can detect when running in Kubernetes
- Successfully captures original cluster DNS server IP
- Preserves search domains from original resolv.conf
**Tests**: 
- Verify KUBERNETES_SERVICE_HOST detection
- Verify resolv.conf parsing extracts correct nameserver
**Status**: Complete

## Stage 2: Split DNS Middleware
**Goal**: Create DNS middleware that routes queries based on domain
**Success Criteria**:
- Middleware correctly identifies cluster domains
- Routes cluster domains to cluster DNS
- Routes external domains to DoT server
**Tests**:
- Test routing for *.cluster.local domains
- Test routing for external domains
- Verify no DNS leaks for external queries
**Status**: Complete

## Stage 3: Configuration and Settings
**Goal**: Add configuration options for Kubernetes split DNS
**Success Criteria**:
- New environment variables for enabling/configuring split DNS
- Backward compatibility with existing configurations
- Clear documentation of new options
**Tests**:
- Test with feature enabled/disabled
- Test custom cluster domain configurations
- Test fallback behavior
**Status**: Complete

## Stage 4: Integration and Testing
**Goal**: Integrate with existing DNS loop and test in Kubernetes
**Success Criteria**:
- Works correctly as Kubernetes sidecar
- No DNS leaks for external queries
- Cluster services remain accessible
**Tests**:
- End-to-end test in Kubernetes cluster
- DNS leak tests
- Performance benchmarks
**Status**: Complete (Unit tests pass, needs real-world testing)

## Technical Details

### Key Components
1. **KubernetesDNSDetector**: Detects K8s environment and extracts cluster DNS config
2. **SplitDNSMiddleware**: Routes queries based on domain suffix
3. **ClusterDNSClient**: Plain DNS client for cluster queries
4. **Configuration**: New settings for enabling and configuring split DNS

### Domain Patterns to Handle
- `*.cluster.local` - All cluster-local domains
- `*.svc.cluster.local` - Service domains
- `*.pod.cluster.local` - Pod domains
- Search domains from original resolv.conf

### Environment Variables
- `DNS_KUBERNETES_SPLIT`: Enable split DNS for Kubernetes (default: auto-detect)
- `DNS_CLUSTER_DOMAINS`: Additional domains to route to cluster DNS
- `DNS_CLUSTER_NAMESERVER`: Override detected cluster DNS server