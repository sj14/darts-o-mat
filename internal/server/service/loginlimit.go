package service

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// ErrTooManyLogins means the username had too many failed logins lately, so even the
// right password isn't tried until the time is over (see Options.MaxLoginFailures).
var ErrTooManyLogins = errors.New("too many failed logins, try again later")

// networkFactor is how many times more failed logins a network may have than a username:
// many people can share one address (a household, an office, a mobile provider's NAT).
const networkFactor = 5

// loginWindow is how long a failed login counts.
const loginWindow = 15 * time.Minute

// loginLimiter counts the failed logins per username in the last loginWindow. It lives in
// memory, like the rest of the server's live state (there is one process), so a restart
// forgets the counts.
type loginLimiter struct {
	max      int
	mu       sync.Mutex
	failures map[string][]time.Time
}

// blocked reports whether the username has used up its failed logins at now. Blocked
// attempts aren't counted, so the block ends loginWindow after the last counted failure
// and can't be extended by trying on.
func (l *loginLimiter) blocked(username string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(username, now)) >= l.max
}

// fail counts a failed login.
func (l *loginLimiter) fail(username string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.failures) >= 1000 { // names that don't exist count as well: forget the old ones
		for name := range l.failures {
			l.recent(name, now)
		}
	}
	l.failures[username] = append(l.recent(username, now), now)
}

// reset forgets the failures of a username after a login that worked.
func (l *loginLimiter) reset(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, username)
}

// recent returns the failures of the username within the window and drops older ones.
// The caller holds l.mu.
func (l *loginLimiter) recent(username string, now time.Time) []time.Time {
	times := l.failures[username]
	i := 0
	for i < len(times) && now.Sub(times[i]) >= loginWindow {
		i++
	}
	if i == len(times) {
		delete(l.failures, username)
		return nil
	}
	l.failures[username] = times[i:]
	return times[i:]
}

// limitLogins limits the failed logins per username to max, and per network to
// networkFactor times as many; 0 removes the limits.
func (s *Service) limitLogins(max int) {
	s.opts.MaxLoginFailures = max
	s.logins, s.networks = nil, nil
	if max > 0 {
		s.logins = &loginLimiter{max: max, failures: map[string][]time.Time{}}
		s.networks = &loginLimiter{max: networkFactor * max, failures: map[string][]time.Time{}}
	}
}

type networkKey struct{}

// WithNetwork returns a context that says which network a login comes from, see
// Service.ClientNetwork; Login limits the failed logins per network, too.
func WithNetwork(ctx context.Context, network string) context.Context {
	return context.WithValue(ctx, networkKey{}, network)
}

func networkOf(ctx context.Context) string {
	n, _ := ctx.Value(networkKey{}).(string)
	return n
}

// ClientNetwork returns the network a request comes from, "" if that isn't known: the
// address of an IPv4 client, the /64 of an IPv6 client (one connection has that many
// addresses). The address is the one the request came from, unless Options.ClientIPHeader
// names a header (of a proxy that sets it, and is the only way in): then its last value,
// which that proxy added. Without the header, loopback, private, link-local and shared
// (100.64.0.0/10) addresses are unknown: they are most likely a proxy, and all visitors
// behind it would count as one.
func (s *Service) ClientNetwork(remoteAddr string, header func(string) string) string {
	var ip netip.Addr
	if name := s.opts.ClientIPHeader; name != "" {
		v := header(name)
		v = strings.TrimSpace(v[strings.LastIndex(v, ",")+1:])
		var err error
		if ip, err = netip.ParseAddr(v); err != nil {
			return ""
		}
	} else {
		ap, err := netip.ParseAddrPort(remoteAddr)
		if err != nil {
			return ""
		}
		ip = ap.Addr().Unmap()
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || shared.Contains(ip) {
			return ""
		}
	}
	ip = ip.Unmap()
	bits := 64
	if ip.Is4() {
		bits = 32
	}
	network, err := ip.Prefix(bits)
	if err != nil {
		return ""
	}
	return network.String()
}

var shared = netip.MustParsePrefix("100.64.0.0/10")
