package service

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
)

// Bus carries the notifications about changes to every replica of the server, this one
// included, so a dart through one replica reaches the pages streaming from another.
// Without one (Options.Bus nil) the notifications stay in the process, which is all a
// single server needs.
type Bus interface {
	// Publish sends the keys of a change to every replica.
	Publish(ctx context.Context, keys ...string) error
	// Listen calls receive with the keys of every notification until ctx is done, and
	// missed whenever notifications may have been lost meanwhile (e.g. after the
	// connection was lost).
	Listen(ctx context.Context, receive func(keys []string), missed func())
}

// The keys of the notifications: a game changed, a tournament changed, the turn of a game
// with a time limit may have changed (its timer has to be set again). Each is followed by
// the ID.
const (
	gameKey       = 'g'
	tournamentKey = 't'
	turnKey       = 'c'
)

func key(kind byte, id int64) string {
	return string(kind) + strconv.FormatInt(id, 10)
}

// publishTimeout is how long publishing a notification may take.
const publishTimeout = 5 * time.Second

// publish tells every replica about a change. The change is stored already, so a failure
// only costs the live updates of the other replicas: this one is told anyway.
func (s *Service) publish(keys ...string) {
	if len(keys) == 0 {
		return
	}
	if s.opts.Bus == nil {
		s.receive(keys)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()
	if err := s.opts.Bus.Publish(ctx, keys...); err != nil {
		log.Printf("publish %q: %v", keys, err)
		s.receive(keys)
	}
}

// Listen receives the notifications of the other replicas (and this one's) from the bus
// until ctx is done; without a bus there is nothing to listen to.
func (s *Service) Listen(ctx context.Context) {
	if s.opts.Bus != nil {
		s.opts.Bus.Listen(ctx, s.receive, s.missed)
	}
}

// receive passes a notification on to the subscribers in this process and sets the turn
// timer of a game with a time limit.
func (s *Service) receive(keys []string) {
	for _, k := range keys {
		if len(k) < 2 {
			continue
		}
		id, err := strconv.ParseInt(k[1:], 10, 64)
		if err != nil {
			log.Printf("unknown notification %q", k)
			continue
		}
		switch k[0] {
		case gameKey:
			s.games.Notify(id)
		case tournamentKey:
			s.tournaments.Notify(id)
		case turnKey:
			s.watchGame(id)
		default:
			log.Printf("unknown notification %q", k)
		}
	}
}

// missed is called when notifications may have been lost: every open page is refreshed
// and the turn timers are set again.
func (s *Service) missed() {
	s.games.NotifyAll()
	s.tournaments.NotifyAll()
	if err := s.ResumeTurnLimits(context.Background()); err != nil {
		log.Printf("set the turn timers: %v", err)
	}
}

// Subscribe returns a channel that receives a value whenever the game changes, e.g. to
// push the new board to open game pages, and a function to stop the subscription.
// Changes that happen while the receiver is busy are merged into one notification.
func (s *Service) Subscribe(gameID int64) (<-chan struct{}, func()) {
	return s.games.Subscribe(gameID)
}

// SubscribeTournament is Subscribe for a tournament: it also changes when one of its
// games does.
func (s *Service) SubscribeTournament(id int64) (<-chan struct{}, func()) {
	return s.tournaments.Subscribe(id)
}

// notify tells the game's subscribers that it changed. It is notifyGame for a change in
// the lobby (an invitation answered or withdrawn), where the caller doesn't have the
// changed game: a game in the lobby has no tournament (matches need no invitations), and
// startGame sets the timer of its first turn.
func (s *Service) notify(gameID int64) {
	s.publish(key(gameKey, gameID))
}

// notifyTournament tells the tournament's subscribers that it changed.
func (s *Service) notifyTournament(id int64) {
	s.publish(key(tournamentKey, id))
}

// notifyGame tells the subscribers of the game, and of its tournament, that it changed,
// and every replica to set the timer for its turn.
func (s *Service) notifyGame(g game.Game) {
	keys := []string{key(gameKey, g.ID)}
	if g.TournamentID != nil {
		keys = append(keys, key(tournamentKey, *g.TournamentID))
	}
	s.publish(append(keys, turnKeys(g)...)...)
}

// turnKeys is the notification that sets the timer of the game's turn on every replica, none
// without a time limit.
func turnKeys(g game.Game) []string {
	if g.TurnSeconds == 0 {
		return nil
	}
	return []string{key(turnKey, g.ID)}
}
