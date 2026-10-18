package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrGuestBusy means a guest can't be deleted while they take part in a running game
	// or tournament.
	ErrGuestBusy = fmt.Errorf("%w: the guest takes part in a running game or tournament", ErrInvalid)
	// ErrWrongPassword is returned when deleting an account with a wrong password.
	ErrWrongPassword = fmt.Errorf("%w: wrong password", ErrInvalid)
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
// account that isn't over or aborted.
func (s *Service) inRunningGame(ctx context.Context, acc account.Account, playerID int64) (bool, error) {
	games, err := s.store.ListGames(ctx, acc.ID, acc.PlayerID, 0)
	if err != nil {
		return false, err
	}
	for _, g := range games {
		if g.Aborted() || !slices.Contains(g.PlayerIDs(), playerID) {
			continue
		}
		v, err := s.view(ctx, acc, g)
		if err != nil {
			return false, err
		}
		if !v.Common().GameOver {
			return true, nil
		}
	}
	tournaments, err := s.store.ListTournaments(ctx, acc.ID, acc.PlayerID, 0)
	if err != nil {
		return false, err
	}
	for _, t := range tournaments {
		if t.Aborted() || !slices.Contains(t.PlayerIDs(), playerID) {
			continue
		}
		v, err := s.tournamentView(ctx, acc, t)
		if err != nil {
			return false, err
		}
		if !v.State.Over {
			return true, nil
		}
	}
	return false, nil
}

// DeleteAccount deletes the account after checking its password: its login, sessions,
// friendships, boards and guests are gone, and the player of the account and its guests
// are shown as "deleted player" in the games that stay as they were. Running games and
// tournaments are dealt with first: the account leaves them with all its players (the
// others play on, or the game is aborted if fewer than two players would be left; a
// tournament ends), and open invitations are declined.
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
	if err := s.leaveEverything(ctx, a); err != nil {
		return err
	}
	return s.store.DeleteAccount(ctx, a)
}

func (s *Service) leaveEverything(ctx context.Context, a account.Account) error {
	games, err := s.store.ListGames(ctx, a.ID, a.PlayerID, 0)
	if err != nil {
		return err
	}
	for _, g := range games {
		if g.Aborted() || g.TournamentID != nil { // tournaments below
			continue
		}
		v, err := s.view(ctx, a, g)
		if err != nil {
			return err
		}
		switch {
		case v.Common().GameOver:
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

	tournaments, err := s.store.ListTournaments(ctx, a.ID, a.PlayerID, 0)
	if err != nil {
		return err
	}
	for _, t := range tournaments {
		if t.Aborted() {
			continue
		}
		v, err := s.tournamentView(ctx, a, t)
		if err != nil {
			return err
		}
		switch {
		case v.State.Over:
		case v.Invited:
			if err = s.store.RemoveTournamentParticipant(ctx, t.ID, a.PlayerID); err == nil {
				s.tournaments.Notify(t.ID)
			}
		default:
			err = s.leaveTournament(ctx, a, t)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// abortTournament aborts a tournament and the games of it that are still running.
func (s *Service) abortTournament(ctx context.Context, t *tournament.Tournament, acc account.Account) error {
	now := time.Now()
	t.AbortedAt = &now
	if err := s.store.AbortTournament(ctx, t); err != nil {
		return err
	}
	games, err := s.store.ListTournamentGames(ctx, t.ID)
	if err != nil {
		return err
	}
	for _, g := range games {
		if g.Aborted() {
			continue
		}
		v, err := s.view(ctx, acc, g)
		if err != nil {
			return err
		}
		if !v.Common().GameOver {
			if err := s.abortGame(ctx, &g); err != nil {
				return err
			}
		}
	}
	s.tournaments.Notify(t.ID)
	return nil
}
