package x01

import (
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

// fields are all fields of the board, in the order a checkout lists its setup darts:
// trebles first, high numbers first.
var fields = func() []throw.Dart {
	var fs []throw.Dart
	for _, m := range []uint8{3, 1} {
		for n := uint8(20); n >= 1; n-- {
			fs = append(fs, throw.Dart{Number: n, Multiplier: m})
		}
	}
	fs = append(fs, throw.Dart{Number: throw.Bull, Multiplier: 1})
	for n := uint8(20); n >= 1; n-- {
		fs = append(fs, throw.Dart{Number: n, Multiplier: 2})
	}
	return append(fs, throw.Dart{Number: throw.Bull, Multiplier: 2})
}()

// finishOrder ranks the numbers to finish on: the common doubles first (D16 can be
// split into D8 and D4 after a miss into the single), the bull last.
var finishOrder = []uint8{20, 16, 8, 18, 12, 10, 14, 4, 6, 2, 19, 17, 15, 13, 11, 9, 7, 5, 3, 1, throw.Bull}

// Checkout suggests how to finish remaining with at most darts darts, nil if that
// isn't possible. For double out it is the usual route of the checkout tables (see
// doubleOutRoute) if the darts left suffice for it. Otherwise, and for the other out
// modes, it searches all routes and prefers fewer darts, then a good finishing field,
// then easy setup darts (singles before trebles, high numbers first).
func (m OutMode) Checkout(remaining uint64, darts int) []throw.Dart {
	if m == OutModeDouble {
		if route := doubleOutRoute(remaining); route != nil && len(route) <= darts {
			return route
		}
	}
	var best, path []throw.Dart
	var bestCost checkoutCost
	var search func(rest uint64)
	search = func(rest uint64) {
		for _, d := range fields {
			score := d.Score()
			if score > rest {
				continue
			}
			path = append(path, d)
			switch {
			case score == rest && m.finishes(d):
				if c := m.cost(path); best == nil || c.less(bestCost) {
					best, bestCost = slices.Clone(path), c
				}
			case score < rest && len(path) < darts && rest-score >= m.minFinish():
				search(rest - score)
			}
			path = path[:len(path)-1]
		}
	}
	search(remaining)
	return best
}

type checkoutCost struct {
	darts, finish, setup int
}

func (c checkoutCost) less(o checkoutCost) bool {
	if c.darts != o.darts {
		return c.darts < o.darts
	}
	if c.finish != o.finish {
		return c.finish < o.finish
	}
	return c.setup < o.setup
}

func (m OutMode) cost(path []throw.Dart) checkoutCost {
	last := path[len(path)-1]
	// Single out prefers to finish with a single, master out with a double.
	class := map[uint8]int{1: 1, 2: 0, 3: 2}[last.Multiplier]
	if m == OutModeSingle {
		class = int(last.Multiplier) - 1
	}
	c := checkoutCost{darts: len(path), finish: class*len(finishOrder) + slices.Index(finishOrder, last.Number)}
	for _, d := range path[:len(path)-1] {
		difficulty := map[uint8]int{1: 1, 3: 2, 2: 3}[d.Multiplier]
		if d.Number == throw.Bull {
			difficulty = 3
		}
		c.setup += difficulty*100 - int(d.Number)
	}
	return c
}
