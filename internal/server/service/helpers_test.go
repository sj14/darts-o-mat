package service

import (
	"context"
	"fmt"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

// The helpers below make a game or tournament the way most tests need it, through the
// lobby: begin with the first of its players (the account's own or a guest), add the
// others, invite the friends, and start it once nobody is waited for. The lobby itself is
// tested in seats_test.go.

func createGame(ctx context.Context, s *Service, acc account.Account, g game.Game, invite []string) (GameView, error) {
	if len(g.Participants) == 0 {
		return GameView{}, fmt.Errorf("%w: no player", ErrInvalid)
	}
	first := g.Participants[0]
	v, err := s.CreateGame(ctx, acc, g, first.PlayerID, first.BoardID)
	if err != nil {
		return v, err
	}
	ref := GameLobby(v.Game.ID)
	for _, p := range g.Participants[1:] {
		if err := s.AddPlayer(ctx, acc, ref, p.PlayerID); err != nil {
			return v, err
		}
		if err := s.SetSeatBoard(ctx, acc, ref, p.PlayerID, p.BoardID); err != nil {
			return v, err
		}
	}
	for _, username := range invite {
		if err := s.Invite(ctx, acc, ref, username); err != nil {
			return v, err
		}
	}
	return startIfReady(ctx, s, acc, v.Game.ID)
}

// startIfReady starts a game that isn't open for strangers once nobody is waited for, as
// a player would.
func startIfReady(ctx context.Context, s *Service, acc account.Account, id int64) (GameView, error) {
	v, err := s.Game(ctx, acc, id)
	if err != nil || v.Started || !v.CanStart || v.Game.Open() {
		return v, err
	}
	if err := s.Start(ctx, acc, GameLobby(id)); err != nil {
		return v, err
	}
	return s.Game(ctx, acc, id)
}

func acceptGame(ctx context.Context, s *Service, acc account.Account, id int64, boardID *int64) (GameView, error) {
	if err := s.Accept(ctx, acc, GameLobby(id), boardID); err != nil {
		return GameView{}, err
	}
	return startIfReady(ctx, s, acc, id)
}

func joinGame(ctx context.Context, s *Service, acc account.Account, id int64, boardID *int64) (GameView, error) {
	if err := s.Join(ctx, acc, GameLobby(id), boardID); err != nil {
		return GameView{}, err
	}
	return s.Game(ctx, acc, id)
}

func withdrawGame(ctx context.Context, s *Service, acc account.Account, id, playerID int64) (GameView, error) {
	if err := s.Withdraw(ctx, acc, GameLobby(id), playerID); err != nil {
		return GameView{}, err
	}
	return startIfReady(ctx, s, acc, id)
}

func createTournament(ctx context.Context, s *Service, acc account.Account, t tournament.Tournament, invite []string) (TournamentView, error) {
	if len(t.Participants) == 0 {
		return TournamentView{}, fmt.Errorf("%w: no player", ErrInvalid)
	}
	first := t.Participants[0]
	v, err := s.CreateTournament(ctx, acc, t, first.PlayerID, first.BoardID)
	if err != nil {
		return v, err
	}
	ref := TournamentLobby(v.Tournament.ID)
	for _, p := range t.Participants[1:] {
		if err := s.AddPlayer(ctx, acc, ref, p.PlayerID); err != nil {
			return v, err
		}
		if err := s.SetSeatBoard(ctx, acc, ref, p.PlayerID, p.BoardID); err != nil {
			return v, err
		}
	}
	for _, username := range invite {
		if err := s.Invite(ctx, acc, ref, username); err != nil {
			return v, err
		}
	}
	return startTournamentIfReady(ctx, s, acc, v.Tournament.ID)
}

func startTournamentIfReady(ctx context.Context, s *Service, acc account.Account, id int64) (TournamentView, error) {
	v, err := s.Tournament(ctx, acc, id)
	if err != nil || v.Started || !v.CanStart || v.Tournament.Open() || len(v.Players) < tournament.MinPlayers {
		return v, err
	}
	if err := s.Start(ctx, acc, TournamentLobby(id)); err != nil {
		return v, err
	}
	return s.Tournament(ctx, acc, id)
}

func acceptTournament(ctx context.Context, s *Service, acc account.Account, id int64, boardID *int64) (TournamentView, error) {
	if err := s.Accept(ctx, acc, TournamentLobby(id), boardID); err != nil {
		return TournamentView{}, err
	}
	return startTournamentIfReady(ctx, s, acc, id)
}

func joinTournament(ctx context.Context, s *Service, acc account.Account, id int64, boardID *int64) (TournamentView, error) {
	if err := s.Join(ctx, acc, TournamentLobby(id), boardID); err != nil {
		return TournamentView{}, err
	}
	return s.Tournament(ctx, acc, id)
}
