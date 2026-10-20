package service

import (
	"context"
	"fmt"
	"math/rand/v2"
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
	// Ready is true once no invitation is pending anymore and no seat is free; only then
	// games can start.
	Ready bool `json:"ready"`

	// The following fields depend on the account the view was created for.
	CanPlay bool `json:"can_play"` // may start the games of its players' matches
	Invited bool `json:"invited"`  // has a pending invitation to answer
	// Mine are the players of the account that take part: its own player and its guests.
	Mine []int64 `json:"-"`
	// Boards are the boards the viewer can pick to accept an invitation on.
	Boards []BoardChoice `json:"boards,omitempty"`
}

// CanStart reports whether the account the view was created for may start the game of
// the match now (see StartMatch): it controls one of the match's players.
func (v TournamentView) CanStart(m tournament.Match) bool {
	return v.CanPlay && v.Ready && m.Ready() && (slices.Contains(v.Mine, m.PlayerIDs[0]) || slices.Contains(v.Mine, m.PlayerIDs[1]))
}

// Pending reports whether the player hasn't answered the invitation to the tournament yet.
func (v TournamentView) Pending(playerID int64) bool {
	p, ok := v.Tournament.Participant(playerID)
	return ok && p.Status == game.InvitationPending
}

// CreateTournament creates a tournament for the account, like CreateGame: the players are
// the account's own player and guests (the Participants, each with the board it throws
// on) plus the invited friends. The order of the players is drawn at random; it seeds
// the bracket.
func (s *Service) CreateTournament(ctx context.Context, acc account.Account, t tournament.Tournament, inviteUsernames []string) (TournamentView, error) {
	t.ID = 0
	t.Settings.Legs, t.Settings.Sets = max(t.Settings.Legs, 1), max(t.Settings.Sets, 1)
	if len(t.Participants) == 0 {
		return TournamentView{}, fmt.Errorf("%w: choose at least one of your players", ErrInvalid)
	}
	if err := s.checkOwnPlayers(ctx, acc, t.PlayerIDs()); err != nil {
		return TournamentView{}, err
	}
	for i := range t.Participants {
		t.Participants[i].Status = game.InvitationAccepted
		if err := needBoard(t.Lobby, t.Participants[i].BoardID); err != nil {
			return TournamentView{}, err
		}
		if err := s.checkBoard(ctx, acc, t.Participants[i].BoardID); err != nil {
			return TournamentView{}, err
		}
	}
	invited, err := s.invitedPlayers(ctx, acc, inviteUsernames)
	if err != nil {
		return TournamentView{}, err
	}
	for _, id := range invited {
		t.Participants = append(t.Participants, tournament.Participant{PlayerID: id, Status: game.InvitationPending})
	}
	rand.Shuffle(len(t.Participants), func(i, j int) { t.Participants[i], t.Participants[j] = t.Participants[j], t.Participants[i] })
	if err := t.Validate(); err != nil {
		return TournamentView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.store.CreateTournament(ctx, &t); err != nil {
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

// AcceptTournamentInvitation accepts the invitation to a tournament. The player throws
// on the board if one is given (one of the account's, or one other players throw on), in
// all the matches they play.
func (s *Service) AcceptTournamentInvitation(ctx context.Context, acc account.Account, id int64, boardID *int64) (TournamentView, error) {
	t, err := s.store.GetTournament(ctx, id)
	if err != nil {
		return TournamentView{}, err
	}
	if err := needBoard(t.Lobby, boardID); err != nil {
		return TournamentView{}, err
	}
	if err := s.checkTournamentBoard(ctx, acc, t, boardID); err != nil {
		return TournamentView{}, err
	}
	if err := s.store.AcceptTournamentInvitation(ctx, id, acc.PlayerID, boardID); err != nil {
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
	if err := s.store.RemoveTournamentParticipant(ctx, id, acc.PlayerID); err != nil {
		return err
	}
	s.tournaments.Notify(id)
	return nil
}

// checkTournamentBoard is checkBoard for a player of a tournament: also a board the other
// players throw on will do.
func (s *Service) checkTournamentBoard(ctx context.Context, acc account.Account, t tournament.Tournament, boardID *int64) error {
	if boardID == nil {
		return nil
	}
	for _, p := range t.Participants {
		if p.BoardID != nil && *p.BoardID == *boardID {
			return nil
		}
	}
	return s.checkBoard(ctx, acc, boardID)
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
	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	for _, p := range g.Participants {
		if p.BoardID == nil {
			continue
		}
		if err := s.checkBoardFree(ctx, *p.BoardID, 0); err != nil {
			return GameView{}, err
		}
	}
	if err := s.completeFinished(ctx, acc); err != nil {
		return GameView{}, err
	}
	if err := s.store.CreateGame(ctx, &g); err != nil {
		return GameView{}, err
	}
	if err := s.takeBoards(ctx, g); err != nil {
		return GameView{}, err
	}
	if err := s.startGame(ctx, g.ID); err != nil {
		return GameView{}, err
	}
	s.tournaments.Notify(id)
	return s.Game(ctx, acc, g.ID)
}

// tournamentView builds the view of a tournament for an account, ErrNotFound if the
// account takes no part in it: no player of the account (its own or a guest) is a
// participant.
func (s *Service) tournamentView(ctx context.Context, acc account.Account, t tournament.Tournament) (TournamentView, error) {
	v := TournamentView{Tournament: t}
	v.Ready = t.Full(len(t.Participants)) && !slices.ContainsFunc(t.Participants, func(p tournament.Participant) bool { return p.Status == game.InvitationPending })
	var err error
	if v.Players, err = s.store.GetPlayers(ctx, t.PlayerIDs()); err != nil {
		return TournamentView{}, err
	}
	accepted := false
	for _, p := range v.Players {
		if p.ID == acc.PlayerID || (p.GuestOf != nil && *p.GuestOf == acc.ID) {
			v.Mine = append(v.Mine, p.ID)
			accepted = accepted || !v.Pending(p.ID)
		}
	}
	if len(v.Mine) == 0 {
		return TournamentView{}, ErrNotFound
	}
	v.Invited = v.Pending(acc.PlayerID) && !t.Aborted()
	v.CanPlay = accepted && !t.Aborted()
	if v.Invited {
		if v.Boards, err = s.tournamentBoardChoices(ctx, acc, t); err != nil {
			return TournamentView{}, err
		}
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
			r.Over, r.WinnerID = g.Completed(darts, c, s.now()), *c.WinnerID
			r.Unconfirmed = !r.Over
			r.Score = [2]int{score[g.PlayerIDs()[0]], score[g.PlayerIDs()[1]]}
			for i := range v.Stats {
				v.Stats[i].Add(state)
			}
		}
		results[g.TournamentMatch] = r
	}
	if t.Full(len(t.Participants)) { // the schedule needs all players
		v.State = tournament.Plan(t.Format, t.PlayerIDs(), results)
	}
	return v, nil
}
