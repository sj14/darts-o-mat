package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

// TournamentView is a tournament with its players and the schedule with the results so
// far.
type TournamentView struct {
	Tournament tournament.Tournament `json:"tournament"`
	Players    []player.Player       `json:"players"` // in the order of the draw
	State      tournament.State      `json:"state"`
	// Stats are the statistics of each player over the finished games of the tournament,
	// in the order of Players.
	Stats []stats.Summary `json:"stats"`
	// Time is when the tournament was played so far: from the first dart of its games
	// to the last.
	Time turn.Span `json:"time"`
	// Started is true once the tournament left the lobby: its order is drawn, its games can
	// start.
	Started bool `json:"started"`
	// CanStart is true in the lobby once no invitation is pending and no seat is free
	// (tournament.Tournament.CanStart); it needs tournament.MinPlayers too.
	CanStart bool `json:"can_start"`

	// The following fields depend on the account the view was created for.
	CanPlay bool `json:"can_play"` // may start the games of its players' matches
	Invited bool `json:"invited"`  // has a pending invitation to answer
	// Mine are the players of the account that take part: its own player and its guests.
	Mine []int64 `json:"-"`
	// Boards are the boards the viewer can pick to accept an invitation on, or for its
	// players in the lobby.
	Boards []BoardChoice `json:"boards,omitempty"`
	// Choices are who the viewer can add to the lobby or invite, only in the lobby.
	Choices *LobbyChoices `json:"choices,omitempty"`
}

// MayStart reports whether the account the view was created for may start the game of
// the match now (see StartMatch): it controls one of the match's players.
func (v TournamentView) MayStart(m tournament.Match) bool {
	return v.CanPlay && v.Started && m.Ready() && (slices.Contains(v.Mine, m.PlayerIDs[0]) || slices.Contains(v.Mine, m.PlayerIDs[1]))
}

// Finished reports whether nothing happens in the tournament anymore: it is over or was
// aborted.
func (v TournamentView) Finished() bool {
	return v.Tournament.Aborted() || v.State.Over
}

// Pending reports whether the player hasn't answered the invitation to the tournament yet.
func (v TournamentView) Pending(playerID int64) bool {
	p, ok := v.Tournament.Participant(playerID)
	return ok && p.Status == game.InvitationPending
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
	ts, err := s.store.ListInvitedTournaments(ctx, acc.PlayerID)
	if err != nil {
		return nil, err
	}
	var views []TournamentView
	for _, t := range ts {
		if t.Aborted() {
			continue
		}
		v, err := s.tournamentView(ctx, acc, t)
		if err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, nil
}

// StartMatch starts the game of a match whose players are known. An account with a
// player of the match may start it; its players accepted the tournament, so they don't
// have to accept the game, and throw on the boards they chose for the tournament.
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
	case !v.Started:
		return GameView{}, ErrNotReady
	case match < 0 || match >= len(v.State.Matches):
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, tournament.ErrNoMatch)
	case !v.State.Matches[match].Ready():
		return GameView{}, fmt.Errorf("%w: match %d can't be started", ErrInvalid, match)
	}
	m := v.State.Matches[match]
	if !v.MayStart(m) {
		return GameView{}, ErrForbidden
	}
	g := v.Tournament.Game(match, m.PlayerIDs[0], m.PlayerIDs[1])
	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	// The match's game seats both players on the boards they chose for the tournament;
	// their seat in this tournament doesn't count. A match doesn't need boards even of a
	// board-only tournament.
	st := tournamentSeat(v.Tournament)
	st.lobby, st.holdsBoards = game.Lobby{}, true
	err = s.takeSeats(ctx, acc, st, g.Participants, id, func() error {
		if err := s.completePrevious(ctx, v.State, m); err != nil {
			return err
		}
		if err := s.store.CreateGame(ctx, &g); err != nil {
			return err
		}
		if err := s.takeBoards(ctx, g); err != nil {
			return err
		}
		return s.startGame(ctx, g.ID)
	})
	if err != nil {
		return GameView{}, err
	}
	s.tournaments.Notify(id)
	return s.Game(ctx, acc, g.ID)
}

// completePrevious completes the games of the tournament that are over but can still be
// corrected and that one of the match's players played in: starting the next match accepts
// them (user's decision 2026-10-20), also for the player who doesn't start it. Whoever won
// them is in the match already. The caller holds s.mu.
func (s *Service) completePrevious(ctx context.Context, st tournament.State, m tournament.Match) error {
	for _, prev := range st.Matches {
		if prev.Result == nil || !prev.Result.Unconfirmed || !slices.ContainsFunc(m.PlayerIDs[:], func(id int64) bool { return slices.Contains(prev.PlayerIDs[:], id) }) {
			continue
		}
		g, err := s.store.GetGame(ctx, prev.Result.GameID)
		if err != nil {
			return err
		}
		if err := s.completeGame(ctx, &g); err != nil {
			return err
		}
	}
	return nil
}

// tournamentView builds the view of a tournament for an account, ErrNotFound if the
// account takes no part in it: no player of the account (its own or a guest) is a
// participant.
func (s *Service) tournamentView(ctx context.Context, acc account.Account, t tournament.Tournament) (TournamentView, error) {
	v, err := s.buildTournament(ctx, t)
	if err != nil {
		return TournamentView{}, err
	}
	pt, err := participationOf(acc, v.Players, v.Pending, t.Aborted())
	if err != nil {
		return TournamentView{}, err
	}
	v.Mine, v.Invited, v.CanPlay = pt.mine, pt.invited, pt.canPlay
	if v.Invited || (v.CanPlay && !t.Started()) {
		if v.Boards, err = s.tournamentBoardChoices(ctx, acc, t); err != nil {
			return TournamentView{}, err
		}
	}
	if v.CanPlay && !t.Started() {
		if v.Choices, err = s.lobbyChoices(ctx, acc, t.PlayerIDs()); err != nil {
			return TournamentView{}, err
		}
	}
	return v, nil
}

// buildTournament computes everything of a tournament's view that doesn't depend on who
// looks at it, like build for a game.
func (s *Service) buildTournament(ctx context.Context, t tournament.Tournament) (TournamentView, error) {
	v := TournamentView{Tournament: t, Started: t.Started(), CanStart: t.CanStart()}
	var err error
	if v.Players, err = s.store.GetPlayers(ctx, t.PlayerIDs()); err != nil {
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
			r.Unconfirmed = !g.Completed(darts, c, s.now())
			r.Score = [2]int{score[g.PlayerIDs()[0]], score[g.PlayerIDs()[1]]}
			for i := range v.Stats {
				v.Stats[i].Add(state)
			}
		}
		results[g.TournamentMatch] = r
	}
	if t.Started() { // the schedule needs all players, drawn
		v.State = tournament.Plan(t.Format, t.PlayerIDs(), results)
	}
	// A completed tournament can't change anymore: whoever looks at it first stores that,
	// like the statistics of a game, so later lookups leave it out in SQL.
	if t.CompletedAt == nil && !t.Aborted() && v.State.Completed() {
		now := s.now()
		v.Tournament.CompletedAt = &now
		if err := s.store.CompleteTournament(ctx, &v.Tournament); err != nil {
			return TournamentView{}, err
		}
	}
	return v, nil
}
