package service

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/store"
)

// ErrTooManyLogins means the username had too many failed logins lately, so even the
// right password isn't tried until the time is over (see Options.MaxLoginFailures).
var ErrTooManyLogins = errors.New("too many failed logins, try again later")

// networkFactor is how many times more failed logins a network may have than a username:
// many people can share one address (a household, an office, a mobile provider's NAT).
const networkFactor = 5

// loginWindow is how long a failed login counts.
const loginWindow = 15 * time.Minute

// loginLimiter counts the failed logins per key (a username or a network) in the last
// loginWindow. They are stored in the database, so all replicas count together and a
// restart doesn't forget them; prefix tells the kinds of keys apart.
type loginLimiter struct {
	store  *store.Store
	prefix string
	max    int
}

// blocked reports whether the key has used up its failed logins at now. Blocked attempts
// aren't counted, so the block ends loginWindow after the last counted failure and can't
// be extended by trying on.
func (l *loginLimiter) blocked(ctx context.Context, key string, now time.Time) (bool, error) {
	n, err := l.store.CountLoginFailures(ctx, l.prefix+key, now.Add(-loginWindow))
	return n >= l.max, err
}

// fail counts a failed login; the failures older than the window are forgotten on the way.
func (l *loginLimiter) fail(ctx context.Context, key string, now time.Time) error {
	return l.store.AddLoginFailure(ctx, l.prefix+key, now, now.Add(-loginWindow))
}

// reset forgets the failures of a key after a login that worked.
func (l *loginLimiter) reset(ctx context.Context, key string) error {
	return l.store.ResetLoginFailures(ctx, l.prefix+key)
}

// limitLogins limits the failed logins per username to max, and per network to
// networkFactor times as many; 0 removes the limits.
func (s *Service) limitLogins(max int) {
	s.opts.MaxLoginFailures = max
	s.logins, s.networks = nil, nil
	if max > 0 {
		s.logins = &loginLimiter{store: s.store, prefix: "user:", max: max}
		s.networks = &loginLimiter{store: s.store, prefix: "network:", max: networkFactor * max}
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
