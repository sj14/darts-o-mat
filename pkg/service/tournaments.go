package service

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/stats"
	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
)

// TournamentView is a tournament with its players, invitations and the schedule with
// the results so far.
type TournamentView struct {
	Tournament  tournament.Tournament   `json:"tournament"`
	Players     []player.Player         `json:"players"` // in the order of the draw
	Invitations []tournament.Invitation `json:"invitations"`
	State       tournament.State        `json:"state"`
	// Stats are the statistics of each player over the finished games of the tournament,
	// in the order of Players.
	Stats []stats.Summary `json:"stats"`
	// Time is when the tournament was played so far: from the first dart of its games
	// to the last.
	Time turn.Span `json:"time"`
	// Ready is true once no invitation is pending anymore; only then games can start.
	Ready bool `json:"ready"`

	// The following fields depend on the account the view was created for.
	IsOwner bool `json:"is_owner"`
	CanPlay bool `json:"can_play"` // may start games: the owner all, players with an account those of their matches
	Invited bool `json:"invited"`  // has a pending invitation to answer

	viewer int64 // the player of the account the view was created for
}

// CanStart reports whether the account the view was created for may start the game of
// the match now (see StartMatch).
func (v TournamentView) CanStart(m tournament.Match) bool {
	return v.CanPlay && v.Ready && m.Ready() && (v.IsOwner || m.PlayerIDs[0] == v.viewer || m.PlayerIDs[1] == v.viewer)
}

// InvitationOf returns the invitation of a player, nil if the player wasn't invited.
func (v TournamentView) InvitationOf(playerID int64) *tournament.Invitation {
	for _, inv := range v.Invitations {
		if inv.PlayerID == playerID {
			return &inv
		}
	}
	return nil
}

// CreateTournament creates a tournament owned by the account, like CreateGame: the
// players are the account's own player and guests plus the invited friends. The order
// of the players is drawn at random; it seeds the bracket.
func (s *Service) CreateTournament(ctx context.Context, acc account.Account, t tournament.Tournament, inviteUsernames []string) (TournamentView, error) {
	t.ID, t.OwnerAccountID = 0, acc.ID
	t.Settings.Legs, t.Settings.Sets = max(t.Settings.Legs, 1), max(t.Settings.Sets, 1)
	if err := s.checkOwnPlayers(ctx, acc, t.PlayerIDs); err != nil {
		return TournamentView{}, err
	}
	invited, err := s.invitedPlayers(ctx, acc, inviteUsernames)
	if err != nil {
		return TournamentView{}, err
	}
	t.PlayerIDs = append(t.PlayerIDs, invited...)
	if err := s.checkBoard(ctx, acc, t.Settings.BoardID); err != nil {
		return TournamentView{}, err
	}
	rand.Shuffle(len(t.PlayerIDs), func(i, j int) { t.PlayerIDs[i], t.PlayerIDs[j] = t.PlayerIDs[j], t.PlayerIDs[i] })
	if err := t.Validate(); err != nil {
		return TournamentView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.store.CreateTournament(ctx, &t, invited); err != nil {
		return TournamentView{}, err
	}
	return s.tournamentView(ctx, acc, t)
}

func (s *Service) Tournament(ctx context.Context, acc account.Account, id int64) (TournamentView, error) {
	t, err := s.store.GetTournament(ctx, id)
	if err != nil {
		return TournamentView{}, err
	}
	return s.tournamentView(ctx, acc, t)
}

// ListTournaments returns the most recent tournaments the account created or was
// invited to, newest first.
func (s *Service) ListTournaments(ctx context.Context, acc account.Account, limit int64) ([]TournamentView, error) {
	ts, err := s.store.ListTournaments(ctx, acc.ID, acc.PlayerID, limit)
	if err != nil {
		return nil, err
	}
	views := []TournamentView{}
	for _, t := range ts {
		v, err := s.tournamentView(ctx, acc, t)
		if err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, nil
}

// TournamentInvitations returns the tournaments the account was invited to and hasn't
// answered yet.
func (s *Service) TournamentInvitations(ctx context.Context, acc account.Account) ([]TournamentView, error) {
	invitations, err := s.store.ListPendingTournamentInvitations(ctx, acc.PlayerID)
	if err != nil {
		return nil, err
	}
	var views []TournamentView
	for _, inv := range invitations {
		v, err := s.Tournament(ctx, acc, inv.TournamentID)
		if err != nil {
			return nil, err
		}
		if !v.Tournament.Aborted() {
			views = append(views, v)
		}
	}
	return views, nil
}

func (s *Service) AcceptTournamentInvitation(ctx context.Context, acc account.Account, id int64) (TournamentView, error) {
	if err := s.store.AcceptTournamentInvitation(ctx, id, acc.PlayerID); err != nil {
		return TournamentView{}, err
	}
	s.tournaments.Notify(id)
	return s.Tournament(ctx, acc, id)
}

// DeclineTournamentInvitation removes the account's player from a tournament it was
// invited to. Its games only start once everyone accepted, so it has none yet.
func (s *Service) DeclineTournamentInvitation(ctx context.Context, acc account.Account, id int64) error {
	v, err := s.Tournament(ctx, acc, id)
	if err != nil {
		return err
	}
	if !v.Invited {
		return ErrNotFound
	}
	if err := s.store.RemoveTournamentPlayer(ctx, id, acc.PlayerID); err != nil {
		return err
	}
	s.tournaments.Notify(id)
	return nil
}

// AbortTournament ends a tournament without a winner. Only the owner may abort it; it
// doesn't count in the statistics, its finished games do.
func (s *Service) AbortTournament(ctx context.Context, acc account.Account, id int64) (TournamentView, error) {
	v, err := s.Tournament(ctx, acc, id)
	if err != nil {
		return TournamentView{}, err
	}
	switch {
	case !v.IsOwner:
		return TournamentView{}, ErrForbidden
	case v.Tournament.Aborted():
		return TournamentView{}, ErrAborted
	case v.State.Over:
		return TournamentView{}, fmt.Errorf("%w: the tournament is over", ErrInvalid)
	}
	now := time.Now()
	v.Tournament.AbortedAt = &now
	if err := s.store.AbortTournament(ctx, &v.Tournament); err != nil {
		return TournamentView{}, err
	}
	s.tournaments.Notify(id)
	return s.tournamentView(ctx, acc, v.Tournament)
}

// StartMatch starts the game of a match whose players are known. The owner of the
// tournament or a player of the match may start it; the game's owner is the owner of
// the tournament, and its players with an account accepted the tournament, so they
// don't have to accept the game.
func (s *Service) StartMatch(ctx context.Context, acc account.Account, id int64, match int) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Tournament(ctx, acc, id)
	if err != nil {
		return GameView{}, err
	}
	switch {
	case v.Tournament.Aborted():
		return GameView{}, ErrAborted
	case !v.CanPlay:
		return GameView{}, ErrForbidden
	case !v.Ready:
		return GameView{}, ErrNotReady
	case match < 0 || match >= len(v.State.Matches):
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, tournament.ErrNoMatch)
	case !v.State.Matches[match].Ready():
		return GameView{}, fmt.Errorf("%w: match %d can't be started", ErrInvalid, match)
	}
	m := v.State.Matches[match]
	if !v.CanStart(m) {
		return GameView{}, ErrForbidden
	}
	g := v.Tournament.Game(match, m.PlayerIDs[0], m.PlayerIDs[1])
	var accounts []int64
	for _, p := range m.PlayerIDs {
		if v.InvitationOf(p) != nil {
			accounts = append(accounts, p)
		}
	}
	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.store.CreateGame(ctx, &g, accounts, game.InvitationAccepted); err != nil {
		return GameView{}, err
	}
	if g.BoardID != nil {
		if err := s.freeBoard(ctx, *g.BoardID, g.ID); err != nil {
			return GameView{}, err
		}
	}
	s.tournaments.Notify(id)
	return s.view(ctx, acc, g)
}

// tournamentView builds the view of a tournament for an account, ErrNotFound if the
// account takes no part in it.
func (s *Service) tournamentView(ctx context.Context, acc account.Account, t tournament.Tournament) (TournamentView, error) {
	invitations, err := s.store.ListTournamentInvitations(ctx, t.ID)
	if err != nil {
		return TournamentView{}, err
	}
	v := TournamentView{Tournament: t, Invitations: invitations, IsOwner: t.OwnerAccountID == acc.ID, viewer: acc.PlayerID}
	v.Ready = t.Full(len(t.PlayerIDs)) && !slices.ContainsFunc(invitations, func(inv tournament.Invitation) bool { return inv.Status == game.InvitationPending })
	var mine *game.InvitationStatus
	if inv := v.InvitationOf(acc.PlayerID); inv != nil {
		mine = &inv.Status
	}
	if v.CanPlay, v.Invited, err = access(v.IsOwner, mine, t.Aborted()); err != nil {
		return TournamentView{}, err
	}

	if v.Players, err = s.store.GetPlayers(ctx, t.PlayerIDs); err != nil {
		return TournamentView{}, err
	}
	for _, p := range v.Players {
		v.Stats = append(v.Stats, stats.Summary{Player: p})
	}

	// The result of each match is that of its newest game that wasn't aborted.
	games, err := s.store.ListTournamentGames(ctx, t.ID)
	if err != nil {
		return TournamentView{}, err
	}
	results := map[int]tournament.Result{}
	for _, g := range games {
		if g.Aborted() {
			continue
		}
		darts, err := s.store.ListDarts(ctx, g.ID)
		if err != nil {
			return TournamentView{}, err
		}
		state, err := g.Play(darts)
		if err != nil {
			return TournamentView{}, err
		}
		r := tournament.Result{GameID: g.ID}
		c := state.Common()
		v.Time = v.Time.Join(c.Times.Game)
		if c.GameOver {
			score := g.Score(state)
			r.Over, r.WinnerID = true, *c.WinnerID
			r.Score = [2]int{score[g.PlayerIDs[0]], score[g.PlayerIDs[1]]}
			for i := range v.Stats {
				v.Stats[i].Add(state)
			}
		}
		results[g.TournamentMatch] = r
	}
	if t.Full(len(t.PlayerIDs)) { // the schedule needs all players
		v.State = tournament.Plan(t.Format, t.PlayerIDs, results)
	}
	return v, nil
}
