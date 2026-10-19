package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sj14/darts-o-mat/server/domain/account"
	"github.com/sj14/darts-o-mat/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/server/domain/throw"
	"github.com/sj14/darts-o-mat/server/domain/tournament"
)

// ErrCantLeave means the game can't be left now, e.g. while the players bull off.
var ErrCantLeave = turn.ErrCantLeave

// LeaveGame lets an account leave a game with all its players, itself and its guests. All
// participants are equal, nobody has to end a game for the others (user's decision
// 2026-10-18; there is no "abort"):
//
//   - In the lobby the players are simply removed, which frees the seats of an open game.
//     The returned view is empty then, the game isn't theirs anymore.
//   - In a running game the others play on without them (a log entry throw.Dart.Leave per
//     player, the legs and sets they won stay theirs), unless fewer than two players
//     would be left: then the game is aborted. A game of one account alone is aborted by
//     leaving it, which is what "Abort game" means there.
//   - Leaving a game of a tournament leaves the tournament, see LeaveTournament.
//
// A player can't leave while bulling off.
func (s *Service) LeaveGame(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	switch {
	case v.Invited:
		return GameView{}, ErrForbidden // decline the invitation instead
	case v.Game.Aborted():
		return GameView{}, ErrAborted
	case v.Common().GameOver:
		return GameView{}, turn.ErrGameOver
	case v.HasLeft:
		return GameView{}, fmt.Errorf("%w: you left the game already", ErrInvalid)
	}
	if v.Game.TournamentID != nil {
		t, err := s.store.GetTournament(ctx, *v.Game.TournamentID)
		if err != nil {
			return GameView{}, err
		}
		if err := s.leaveTournament(ctx, acc, t); err != nil {
			return GameView{}, err
		}
		return s.Game(ctx, acc, gameID)
	}
	return s.leave(ctx, acc, v)
}

// leave is LeaveGame after the checks. The caller holds s.mu.
func (s *Service) leave(ctx context.Context, acc account.Account, v GameView) (GameView, error) {
	gameID := v.Game.ID
	if !v.Ready { // still in the lobby
		for _, id := range v.Mine {
			if err := s.store.RemoveParticipant(ctx, gameID, id); err != nil {
				return GameView{}, err
			}
		}
		s.notifyGame(v.Game)
		return GameView{}, nil
	}

	var leaving []int64
	others := 0
	for _, id := range v.Game.PlayerIDs() {
		switch {
		case v.Common().Records[id].Left:
		case v.IsMine(id):
			leaving = append(leaving, id)
		default:
			others++
		}
	}
	if others < 2 {
		if err := s.abortGame(ctx, &v.Game); err != nil {
			return GameView{}, err
		}
		return s.view(ctx, acc, v.Game)
	}
	entries := make([]throw.Dart, len(leaving))
	log := v.Darts
	for i, id := range leaving {
		entries[i] = throw.Dart{GameID: gameID, PlayerID: id, Leave: true}
		log = append(log, entries[i])
	}
	if _, err := v.Game.Play(log); err != nil {
		if errors.Is(err, turn.ErrCantLeave) {
			return GameView{}, err
		}
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	for i := range entries {
		if err := s.store.AddDart(ctx, &entries[i]); err != nil {
			return GameView{}, err
		}
	}
	s.notifyGame(v.Game)
	return s.view(ctx, acc, v.Game)
}

// LeaveTournament lets an account leave a tournament with all its players. In the lobby
// they are removed (the returned view is empty then); once it is running, it ends for everyone (user's decision
// 2026-10-18): it is aborted with its running games, its finished games stay.
func (s *Service) LeaveTournament(ctx context.Context, acc account.Account, id int64) (TournamentView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.store.GetTournament(ctx, id)
	if err != nil {
		return TournamentView{}, err
	}
	v, err := s.tournamentView(ctx, acc, t)
	if err != nil {
		return TournamentView{}, err
	}
	switch {
	case v.Invited:
		return TournamentView{}, ErrForbidden // decline the invitation instead
	case t.Aborted():
		return TournamentView{}, ErrAborted
	case v.State.Over:
		return TournamentView{}, fmt.Errorf("%w: the tournament is over", ErrInvalid)
	}
	if err := s.leaveTournament(ctx, acc, t); err != nil {
		return TournamentView{}, err
	}
	// Read it again: leaving may have aborted it. From the lobby the account is no longer in
	// it and has nothing to see, so the view is empty, like LeaveGame's.
	v, err = s.Tournament(ctx, acc, id)
	if errors.Is(err, ErrNotFound) {
		return TournamentView{}, nil
	}
	return v, err
}

// leaveTournament is the leaving of a tournament that isn't over. The caller holds s.mu.
func (s *Service) leaveTournament(ctx context.Context, acc account.Account, t tournament.Tournament) error {
	v, err := s.tournamentView(ctx, acc, t)
	if err != nil {
		return err
	}
	if !v.Ready { // still in the lobby
		for _, id := range v.Mine {
			if err := s.store.RemoveTournamentParticipant(ctx, t.ID, id); err != nil {
				return err
			}
		}
		s.tournaments.Notify(t.ID)
		return nil
	}
	return s.abortTournament(ctx, &t)
}
