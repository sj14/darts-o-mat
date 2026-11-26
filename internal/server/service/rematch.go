package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

// ErrNoRematch means the game can't have a rematch, or not one started by the viewer: see
// GameView.CanRematch.
var ErrNoRematch = fmt.Errorf("%w: this game can't have a rematch", ErrInvalid)

// CanRematch reports whether the viewer may start a rematch of the game (Rematch): it is
// over and wasn't aborted, isn't a match of a tournament, has no rematch yet, and the
// viewer played in it to the end. False for a view that didn't look up the rematch (the
// views of a list), since it doesn't know.
func (v GameView) CanRematch() bool {
	return v.rematchKnown && v.Common().GameOver && !v.Game.Aborted() && v.Game.TournamentID == nil &&
		!v.rematched && len(v.Mine) > 0 && !v.HasLeft && !v.Invited
}

// Rematch starts a rematch of a finished game: a new game with the same settings, not
// open for strangers. The viewer's players that didn't leave sit in it on the boards they
// threw on; the other players with an account that didn't leave are invited, friends or
// not, since they just played together (other accounts' guests and bots aren't: their account adds
// them in the lobby). Without anyone to invite it starts at once, else a player starts it
// once everyone accepted, as any lobby. A game has one rematch: the old game's page links
// to it (GameView.RematchID), and asking for another returns it.
func (s *Service) Rematch(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	unlock, err := s.lock(ctx, "Rematch")
	if err != nil {
		return GameView{}, err
	}
	defer unlock()
	old, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	if old.RematchID != 0 { // another player was quicker
		return s.Game(ctx, acc, old.RematchID)
	}
	if !old.CanRematch() {
		return GameView{}, ErrNoRematch
	}
	g := game.Game{Settings: old.Game.Settings, RematchOf: &gameID}
	g.SettingsID = 0 // the rematch gets its own row, like every game that isn't a match
	var invited []game.Participant
	for _, p := range old.Players {
		switch {
		case p.Deleted() || old.Common().Records[p.ID].Left:
		case old.IsMine(p.ID):
			seat, _ := old.Game.Participant(p.ID)
			id := p.ID
			if seat.BotSpreadUM > 0 { // any free bot of the account plays at the old one's strength
				if id, err = s.freeBot(ctx, acc, g.PlayerIDs()); err != nil {
					return GameView{}, err
				}
			}
			g.Participants = append(g.Participants, game.Participant{PlayerID: id, Status: game.InvitationAccepted, BoardID: seat.BoardID, BotSpreadUM: seat.BotSpreadUM})
		case !p.IsGuest():
			invited = append(invited, game.Participant{PlayerID: p.ID, Status: game.InvitationPending})
		}
	}
	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.checkOwnPlayers(ctx, acc, g.PlayerIDs()); err != nil {
		return GameView{}, err
	}
	// The boards of the old game stay shared: a player who threw on a friend's board keeps
	// it, its owner accepts on it.
	st := gameSeat(old.Game)
	st.lobby, st.game = game.Lobby{}, 0
	err = s.takeSeats(ctx, acc, st, g.Participants, 0, func() error {
		g.Participants = append(g.Participants, invited...)
		if err := s.store.CreateGame(ctx, &g); err != nil {
			return err
		}
		if err := s.takeBoards(ctx, g); err != nil {
			return err
		}
		if len(invited) == 0 {
			return s.startGame(ctx, g.ID)
		}
		return nil
	})
	if err != nil {
		return GameView{}, err
	}
	s.notify(gameID)
	s.notify(g.ID)
	return s.Game(ctx, acc, g.ID)
}

// setRematch fills in the rematch of a finished game for the view: RematchID if the
// viewer takes part in it, and whether there is one at all.
func (s *Service) setRematch(ctx context.Context, v *GameView) error {
	v.rematchKnown = true
	if !v.Common().GameOver || v.Game.TournamentID != nil {
		return nil
	}
	r, err := s.store.GetRematch(ctx, v.Game.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	v.rematched = true
	if slices.ContainsFunc(r.PlayerIDs(), v.IsMine) {
		v.RematchID = r.ID
	}
	return nil
}
