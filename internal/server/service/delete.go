package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/store"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrGuestBusy means a guest can't be deleted while they take part in a running game
	// or tournament.
	ErrGuestBusy = fmt.Errorf("%w: the guest takes part in a running game or tournament", ErrInvalid)
	// ErrWrongPassword is returned when deleting an account with a wrong password.
	ErrWrongPassword = fmt.Errorf("%w: wrong password", ErrInvalid)
	// ErrLastAdmin means the last admin can't delete their account or give up the role: the
	// server would be left without one.
	ErrLastAdmin = store.ErrLastAdmin
)

// DeleteGuest deletes a guest player of the account: it is gone from the lists and the
// statistics of the account, and shown as "deleted player" in the games it played, which
// stay as they were (user's decision 2026-10-18: the others' statistics don't change). It
// is refused while the guest is in a running game or tournament (ErrGuestBusy).
func (s *Service) DeleteGuest(ctx context.Context, acc account.Account, playerID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	players, err := s.store.ListPlayers(ctx, acc.ID, acc.PlayerID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(players, func(p player.Player) bool { return p.ID == playerID && p.IsGuest() }) {
		return ErrNotFound
	}
	busy, err := s.inRunningGame(ctx, acc, playerID)
	if err != nil {
		return err
	}
	if busy {
		return ErrGuestBusy
	}
	return s.store.AnonymizePlayer(ctx, playerID)
}

// inRunningGame reports whether the player takes part in a game or tournament of the
// account that isn't finished.
func (s *Service) inRunningGame(ctx context.Context, acc account.Account, playerID int64) (bool, error) {
	games, tournaments, err := s.unfinished(ctx, acc)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(games, func(v GameView) bool { return slices.Contains(v.Game.PlayerIDs(), playerID) }) ||
		slices.ContainsFunc(tournaments, func(v TournamentView) bool { return slices.Contains(v.Tournament.PlayerIDs(), playerID) }), nil
}

// unfinished returns the views of the account's games and tournaments that aren't
// finished: neither over nor aborted.
func (s *Service) unfinished(ctx context.Context, acc account.Account) ([]GameView, []TournamentView, error) {
	games, err := s.store.ListGames(ctx, acc.ID, acc.PlayerID, 0)
	if err != nil {
		return nil, nil, err
	}
	var gameViews []GameView
	for _, g := range games {
		v, err := s.view(ctx, acc, g)
		if err != nil {
			return nil, nil, err
		}
		if !v.Finished() {
			gameViews = append(gameViews, v)
		}
	}
	tournaments, err := s.store.ListTournaments(ctx, acc.ID, acc.PlayerID, 0)
	if err != nil {
		return nil, nil, err
	}
	var tournamentViews []TournamentView
	for _, t := range tournaments {
		v, err := s.tournamentView(ctx, acc, t)
		if err != nil {
			return nil, nil, err
		}
		if !v.Finished() {
			tournamentViews = append(tournamentViews, v)
		}
	}
	return gameViews, tournamentViews, nil
}

// DeleteAccount deletes the account after checking its password: its login, sessions,
// friendships, boards and guests are gone, and the player of the account and its guests
// are shown as "deleted player" in the games that stay as they were. Running games and
// tournaments are dealt with first: the account leaves them with all its players (the
// others play on, or the game is aborted if fewer than two players would be left; a
// tournament ends), and open invitations are declined. The last admin can't delete their
// account (ErrLastAdmin).
func (s *Service) DeleteAccount(ctx context.Context, acc account.Account, password string) error {
	a, err := s.store.GetAccount(ctx, acc.ID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) != nil {
		return ErrWrongPassword
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Asked before leaving the games; the store asks again in its transaction, in case
	// another admin gives up the role meanwhile.
	switch last, err := s.LastAdmin(ctx, a); {
	case err != nil:
		return err
	case last:
		return ErrLastAdmin
	}
	if err := s.leaveEverything(ctx, a); err != nil {
		return err
	}
	return s.store.DeleteAccount(ctx, a)
}

// LastAdmin says whether the account is the only admin, who can't delete their account or
// give up the role. It is for the pages and to refuse early; the store makes sure of it
// when it changes the role or deletes the account.
func (s *Service) LastAdmin(ctx context.Context, acc account.Account) (bool, error) {
	if !acc.Admin {
		return false, nil
	}
	other, err := s.store.OtherAdmin(ctx, acc.ID)
	return !other, err
}

func (s *Service) leaveEverything(ctx context.Context, a account.Account) error {
	games, tournaments, err := s.unfinished(ctx, a)
	if err != nil {
		return err
	}
	for _, v := range games {
		g := v.Game
		switch {
		case g.TournamentID != nil: // left with its tournament below
		case v.Invited:
			if err = s.store.RemoveParticipant(ctx, g.ID, a.PlayerID); err == nil {
				s.notifyGame(g)
			}
		default:
			if _, err = s.leave(ctx, a, v); err != nil { // e.g. while bulling off
				err = s.abortGame(ctx, &g)
			}
		}
		if err != nil {
			return err
		}
	}
	for _, v := range tournaments {
		t := v.Tournament
		if v.Invited {
			if err = s.store.RemoveTournamentParticipant(ctx, t.ID, a.PlayerID); err == nil {
				s.notifyTournament(t.ID)
			}
		} else {
			err = s.leaveTournament(ctx, a, t, false)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// abortTournament aborts a tournament and the games of it that are still running.
func (s *Service) abortTournament(ctx context.Context, t *tournament.Tournament) error {
	now := s.now()
	t.AbortedAt = &now
	if err := s.store.AbortTournament(ctx, t); err != nil {
		return err
	}
	games, err := s.store.ListTournamentGames(ctx, t.ID)
	if err != nil {
		return err
	}
	for _, g := range games {
		v, err := s.build(ctx, g)
		if err != nil {
			return err
		}
		if !v.Finished() {
			if err := s.abortGame(ctx, &g); err != nil {
				return err
			}
		}
	}
	s.notifyTournament(t.ID)
	return nil
}
