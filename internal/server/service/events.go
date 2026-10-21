package service

import "github.com/sj14/darts-o-mat/internal/server/domain/game"

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
	s.games.Notify(gameID)
}

// notifyGame tells the subscribers of the game, and of its tournament, that it changed, and sets the timer
// for its turn.
func (s *Service) notifyGame(g game.Game) {
	s.games.Notify(g.ID)
	if g.TournamentID != nil {
		s.tournaments.Notify(*g.TournamentID)
	}
	s.watchTurn(g)
}
