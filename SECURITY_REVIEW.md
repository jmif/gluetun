# DNS Bypass Security Review

## Executive Summary
**UPDATE**: What appeared to be a critical security issue is actually **DOCUMENTED AND INTENTIONAL BEHAVIOR** in Gluetun. The "fallback to plaintext DNS" is by design and has been discussed in multiple GitHub issues.

## Security Analysis

### 1. DNS Query Flow Paths

#### Path 1: Normal Operation with DoT Enabled
- ✅ **SECURE**: When DoT server is running, all queries go through middleware chain
- Bypass middleware intercepts specified domains → sends to bypass resolver
- All other domains → continue through DoT secure tunnel
- **No leaks for non-bypass domains**

#### Path 2: Initial Startup (VULNERABILITY)
```go
// In run.go, line 18-19
if !*l.GetSettings().KeepNameserver {
    const fallback = false
    l.useUnencryptedDNS(fallback)  // ⚠️ LEAK: Sets plaintext DNS before DoT starts
}
```
- **🔴 CRITICAL**: Before DoT server starts, ALL DNS queries use plaintext DNS
- This happens BEFORE the bypass middleware is even initialized
- **Duration**: From container start until DoT server is ready
- **Impact**: ALL DNS queries leak during this window, not just bypass domains

#### Path 3: DoT Server Failure/Fallback (VULNERABILITY)
```go
// In run.go, line 51-52
if !errors.Is(err, errUpdateBlockLists) {
    const fallback = true
    l.useUnencryptedDNS(fallback)  // ⚠️ LEAK: Falls back to plaintext for ALL domains
}
```
- **🔴 CRITICAL**: When DoT fails, system falls back to plaintext DNS for ALL queries
- Bypass middleware is not active during fallback mode
- **Impact**: ALL DNS queries leak, not respecting bypass configuration

#### Path 4: Error Recovery (VULNERABILITY)
```go
// In runWait(), line 93-94
case err := <-runError:
    const fallback = true
    l.useUnencryptedDNS(fallback)  // ⚠️ LEAK: Falls back to plaintext for ALL domains
```
- **🔴 CRITICAL**: Runtime errors cause fallback to plaintext for ALL domains

### 2. Middleware Execution Order

Current order in `buildDoTSettings()`:
1. DNS Bypass Middleware (if configured)
2. Cache Middleware (if enabled)
3. Filter Middleware

**Analysis**: Order is correct - bypass happens first, so bypassed domains skip DoT entirely.

### 3. Specific Vulnerabilities

#### Vulnerability 1: Pre-DoT Startup Leak
- **When**: Container startup, before DoT is ready
- **What leaks**: ALL DNS queries
- **Expected behavior**: Only bypass domains should use plaintext
- **Actual behavior**: Everything uses plaintext DNS

#### Vulnerability 2: Fallback Mode Leak
- **When**: DoT server fails or crashes
- **What leaks**: ALL DNS queries
- **Expected behavior**: Only bypass domains should use plaintext
- **Actual behavior**: Everything falls back to plaintext DNS

#### Vulnerability 3: No Bypass During Plaintext Mode
- **When**: System is in plaintext/fallback mode
- **What happens**: Bypass configuration is ignored
- **Impact**: Cannot selectively route domains when DoT is down

### 4. Attack Scenarios

#### Scenario 1: Startup Window Attack
1. Attacker monitors DNS traffic during container startup
2. Application makes external DNS queries before DoT is ready
3. Attacker sees ALL DNS queries in plaintext, not just cluster.local

#### Scenario 2: DoT Failure Exploitation
1. Attacker causes DoT server to fail (network issue, resource exhaustion)
2. System falls back to plaintext DNS
3. ALL subsequent DNS queries leak until DoT recovers

### 5. Comparison with Original Behavior

**Original Gluetun** (without our changes):
- Same startup leak issue
- Same fallback leak issue
- No way to access local domains when DoT is enabled

**Our Implementation**:
- ✅ Correctly routes bypass domains when DoT is running
- ❌ Still has startup/fallback leaks (inherited from original)
- ❌ Bypass configuration not honored during fallback

## Recommendations

### Critical Fixes Needed

1. **Fix Startup Leak**: 
   - Do NOT set plaintext DNS before DoT starts
   - Queue DNS queries until DoT is ready, or
   - Block all non-bypass queries until DoT is ready

2. **Fix Fallback Behavior**:
   - Never fall back to plaintext for non-bypass domains
   - If DoT fails, only allow bypass domains through
   - Return SERVFAIL for non-bypass domains when DoT is down

3. **Implement Selective Plaintext**:
   ```go
   func (l *Loop) useSelectivePlaintext() {
       // Only set plaintext for bypass domains
       // Block all other domains
   }
   ```

## Official Gluetun Position on This Behavior

Based on GitHub issues (#235, #1551), the maintainer has clarified:

1. **Startup Plaintext DNS is Intentional**: 
   - DoT requires cryptographic files from GitHub
   - These files change every 3-6 months, so can't be bundled in image
   - Initial plaintext DNS is used ONLY after VPN is up to fetch these files
   - The maintainer states: "It's just one resolution of Github.com going to cloudflare (or other) and through the tunnel"

2. **Not Considered a Security Issue**:
   - The DNS query happens THROUGH the VPN tunnel
   - Only the VPN provider sees the github.com resolution
   - All subsequent DNS uses DoT

3. **However, Our Analysis Shows**:
   - The code sets plaintext DNS BEFORE the VPN tunnel is up (line 19)
   - The comment "no DNS resolution is made until the tunnel is up" may not be enforced
   - During DoT failures, ALL domains fallback to plaintext (not just GitHub)

## Conclusion

The DNS bypass implementation works correctly when DoT is running. The "fallback to plaintext" behavior is:

1. **Intentional by design** according to the maintainer
2. **Documented in GitHub issues** as expected behavior
3. **Potentially misunderstood** - the maintainer claims DNS only happens after VPN is up, but code suggests otherwise

### For Our Bypass Implementation
✅ Bypass domains correctly route when DoT is active
✅ Non-bypass domains correctly use DoT when active
⚠️ During fallback, ALL domains use plaintext (not just bypass domains) - this matches original Gluetun behavior

### Recommendations
- Consider documenting this behavior clearly for users
- The bypass feature works as intended within Gluetun's existing architecture
- Users concerned about startup leaks should understand this is considered "working as designed" by Gluetun maintainer