package camapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

const (
	// giveUpAfter is how long after a detection it is still sent (user's decision
	// 2026-10-20): a dart that arrives later may belong to a turn that is over, or the
	// player entered it by hand meanwhile. It covers a short restart of the server.
	giveUpAfter = 5 * time.Second
	// firstPause is the pause after the first try, doubled after each further one: the
	// tries are at 0, 1 and 3 s.
	firstPause = time.Second
	// maxWaiting is how many reports may wait for the server; more are dropped. Within
	// giveUpAfter a board has a few darts and takeouts at most.
	maxWaiting = 10
)

var (
	// errGaveUp means the server couldn't be reached within giveUpAfter.
	errGaveUp = errors.New("gave up")
	// errStopped means the detection stopped before the report was sent.
	errStopped = errors.New("detection stopped")
	// errTooMany means too many reports were waiting for the server already.
	errTooMany = errors.New("too many reports waiting")
)

// report is a detection to send to the server.
type report struct {
	event   int // the Event's ID, to show what became of it
	at      time.Time
	takeout bool
	dart    dartboard.Hit // unless takeout
}

// reporter sends a board's detections to the server in the order they happened, one at a
// time, so the detection doesn't wait for the server (user's wish 2026-10-20). While the
// server certainly didn't get it (errUnreachable, e.g. while it restarts) it sends the same
// report again, with doubling pauses, until giveUpAfter. Everything else is final: an
// answer of the server (no running game, …), and a timeout or a connection lost on the
// way, after which the server may have entered the dart already (user's decision
// 2026-10-20: rather "not entered" shown wrongly than a dart counted twice, or one the
// player took back entered again).
type reporter struct {
	name   string // of the board, for the log
	server *server
	queue  chan report
	// result is told what became of a report; sending is true while it waits for another try.
	result func(r report, game int64, err error, sending bool)
	now    func() time.Time

	firstPause, giveUpAfter time.Duration // the constants, shorter in tests
}

func newReporter(name string, s *server, result func(r report, game int64, err error, sending bool)) *reporter {
	return &reporter{
		name: name, server: s, queue: make(chan report, maxWaiting), result: result, now: time.Now,
		firstPause: firstPause, giveUpAfter: giveUpAfter,
	}
}

// add queues a report; it doesn't wait.
func (r *reporter) add(rep report) {
	select {
	case r.queue <- rep:
	default:
		r.done(rep, 0, errTooMany)
	}
}

// run sends the queued reports until ctx is done; what is left then isn't sent.
func (r *reporter) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for {
				select {
				case rep := <-r.queue:
					r.done(rep, 0, errStopped)
				default:
					return
				}
			}
		case rep := <-r.queue:
			r.send(ctx, rep)
		}
	}
}

// send sends one report, again while the server can't be reached.
func (r *reporter) send(ctx context.Context, rep report) {
	pause := r.firstPause
	for {
		var game int64
		var err error
		if rep.takeout {
			game, err = r.server.takeout(ctx)
		} else {
			game, err = r.server.dart(ctx, rep.dart)
		}
		switch {
		case ctx.Err() != nil:
			r.done(rep, 0, errStopped)
			return
		case !errors.Is(err, errUnreachable):
			r.done(rep, game, err)
			return
		case r.now().Add(pause).Sub(rep.at) > r.giveUpAfter:
			r.done(rep, 0, fmt.Errorf("%w after %s: %w", errGaveUp, r.giveUpAfter, err))
			return
		}
		log.Printf("%s: event %d: %v; trying again in %s", r.name, rep.event, err, pause)
		r.result(rep, 0, err, true)
		select {
		case <-ctx.Done():
			r.done(rep, 0, errStopped)
			return
		case <-time.After(pause):
		}
		pause *= 2
	}
}

func (r *reporter) done(rep report, game int64, err error) {
	if err != nil {
		log.Printf("%s: event %d not reported: %v", r.name, rep.event, err)
	} else if game != 0 {
		log.Printf("%s: event %d entered in game %d, %s after its detection", r.name, rep.event, game, r.now().Sub(rep.at).Round(time.Millisecond))
	}
	r.result(rep, game, err, false)
}
