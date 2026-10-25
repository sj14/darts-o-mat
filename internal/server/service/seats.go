package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

// Every game and tournament begins as a lobby: whoever creates it sits in it with their
// own player; the players of the account (its own and its guests) are added there,
// friends invited, strangers join an open one. A player starts it, or the last seat of an
// open one does; the throwing order (the draw of a tournament) is drawn then. Every way
// to a seat goes through takeSeats, the one place that checks a seat.

// ErrNotReady means the game or tournament can't start yet: an invitation is pending or a
// seat is free; or it hasn't started yet, so nothing can be thrown or played.
var ErrNotReady = fmt.Errorf("%w: not everyone invited has accepted yet", ErrInvalid)

// ErrStarted means the game or tournament left the lobby: its players can't change anymore.
var ErrStarted = fmt.Errorf("%w: it has started already", ErrInvalid)

// LobbyRef names a game or a tournament, for what both do in their lobby.
type LobbyRef struct {
	Tournament bool
	ID         int64
}

// GameLobby and TournamentLobby name the lobby of a game and of a tournament.
func GameLobby(id int64) LobbyRef       { return LobbyRef{ID: id} }
func TournamentLobby(id int64) LobbyRef { return LobbyRef{Tournament: true, ID: id} }

// lobby is what the seat functions need of a game or tournament, whichever it is.
type lobby struct {
	ref LobbyRef
	game.Lobby
	seats   []game.Participant // player, answer and board; the game ID only for a game
	players []player.Player
	started bool
	aborted bool
}

func (l lobby) pending(playerID int64) bool {
	i := slices.IndexFunc(l.seats, func(p game.Participant) bool { return p.PlayerID == playerID })
	return i >= 0 && l.seats[i].Status == game.InvitationPending
}

func (l lobby) statuses() []game.InvitationStatus {
	statuses := make([]game.InvitationStatus, len(l.seats))
	for i, p := range l.seats {
		statuses[i] = p.Status
	}
	return statuses
}

// seat is where a new player sits, for checkSeatBoard: the boards of the others are
// shared, and a game takes the board it is played on.
func (l lobby) seat() seat {
	st := seat{lobby: l.Lobby, holdsBoards: !l.ref.Tournament}
	if !l.ref.Tournament {
		st.game = l.ref.ID
	}
	for _, p := range l.seats {
		if p.BoardID != nil {
			st.shared = append(st.shared, *p.BoardID)
		}
	}
	return st
}

// LobbyChoices are who the viewer can bring into a lobby: the account's players that aren't
// in it (AddPlayer) and its friends that aren't (Invite).
type LobbyChoices struct {
	Players []player.Player `json:"players"`
	Friends []Friend        `json:"friends"`
	// HasFriends is false if the account has no friends at all (else they are in already).
	HasFriends bool `json:"has_friends"`
}

// lobbyChoices lists who of the account's players and friends isn't seated yet.
func (s *Service) lobbyChoices(ctx context.Context, acc account.Account, seated []int64) (*LobbyChoices, error) {
	players, err := s.store.ListPlayers(ctx, acc.ID, acc.PlayerID)
	if err != nil {
		return nil, err
	}
	friends, err := s.Friends(ctx, acc)
	if err != nil {
		return nil, err
	}
	c := &LobbyChoices{Players: []player.Player{}, Friends: []Friend{}, HasFriends: len(friends.Friends) > 0}
	for _, p := range players {
		if !slices.Contains(seated, p.ID) {
			c.Players = append(c.Players, p)
		}
	}
	for _, f := range friends.Friends {
		if !slices.Contains(seated, f.PlayerID) {
			c.Friends = append(c.Friends, f)
		}
	}
	return c, nil
}

// loadLobby loads the game or tournament with its players.
func (s *Service) loadLobby(ctx context.Context, ref LobbyRef) (lobby, error) {
	l := lobby{ref: ref}
	if ref.Tournament {
		t, err := s.store.GetTournament(ctx, ref.ID)
		if err != nil {
			return lobby{}, err
		}
		l.Lobby, l.started, l.aborted = t.Lobby, t.Started(), t.Aborted()
		for _, p := range t.Participants {
			l.seats = append(l.seats, game.Participant{PlayerID: p.PlayerID, Status: p.Status, BoardID: p.BoardID})
		}
	} else {
		g, err := s.store.GetGame(ctx, ref.ID)
		if err != nil {
			return lobby{}, err
		}
		l.Lobby, l.seats, l.started, l.aborted = g.Lobby, g.Participants, g.Started(), g.Aborted()
	}
	var err error
	if l.players, err = s.store.GetPlayers(ctx, playerIDs(l.seats)); err != nil {
		return lobby{}, err
	}
	return l, nil
}

func playerIDs(seats []game.Participant) []int64 {
	ids := make([]int64, len(seats))
	for i, p := range seats {
		ids[i] = p.PlayerID
	}
	return ids
}

// lobbyFor loads the lobby for an account that takes part in it (ErrNotFound otherwise,
// so it isn't revealed) and refuses one that started or was aborted.
func (s *Service) lobbyFor(ctx context.Context, acc account.Account, ref LobbyRef) (lobby, participation, error) {
	l, err := s.loadLobby(ctx, ref)
	if err != nil {
		return lobby{}, participation{}, err
	}
	pt, err := participationOf(acc, l.players, l.pending, l.aborted)
	switch {
	case err != nil:
		return lobby{}, participation{}, err
	case l.aborted:
		return lobby{}, participation{}, ErrAborted
	case l.started:
		return lobby{}, participation{}, ErrStarted
	}
	return l, pt, nil
}

// takeSeats is the one check of every way to a seat: creating a game or tournament (its
// first seat), adding a player of the account, accepting an invitation, joining an open
// one and starting a match. For each seat: its board (checkSeatBoard), and that the
// player has no seat elsewhere (checkPlayersFree; the tournament keep doesn't count).
// Then the games the account finished are completed (going on accepts their result),
// and write stores the seats. The caller holds s.mu.
func (s *Service) takeSeats(ctx context.Context, acc account.Account, st seat, seats []game.Participant, keep int64, write func() error) error {
	for _, p := range seats {
		if err := s.checkSeatBoard(ctx, acc, st, p.BoardID); err != nil {
			return err
		}
	}
	if err := s.checkPlayersFree(ctx, acc, playerIDs(seats), keep); err != nil {
		return err
	}
	if err := s.completeFinished(ctx, acc); err != nil {
		return err
	}
	return write()
}

// CreateGame creates a game in its lobby, with the settings of g (and its Lobby: seats for
// strangers, only with boards), and a player of the account in it (0: its own player; a
// guest, e.g. while the account's own player plays on another board), on the board if one
// is given. Its other players are added and friends invited in the lobby (AddPlayer,
// Invite), then a player starts it (Start). Nobody owns the game: the account has no more
// rights in it than the others.
func (s *Service) CreateGame(ctx context.Context, acc account.Account, g game.Game, playerID int64, boardID *int64) (GameView, error) {
	g.ID, g.StartedAt, g.CompletedAt, g.AbortedAt, g.TournamentID = 0, nil, nil, nil, nil
	g.Settings = g.Settings.WithDefaults()
	g.Participants = []game.Participant{{PlayerID: firstPlayer(acc, playerID), Status: game.InvitationAccepted, BoardID: boardID}}
	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOwnPlayers(ctx, acc, g.PlayerIDs()); err != nil {
		return GameView{}, err
	}
	err := s.takeSeats(ctx, acc, seat{lobby: g.Lobby, holdsBoards: true}, g.Participants, 0, func() error {
		if err := s.store.CreateGame(ctx, &g); err != nil {
			return err
		}
		return s.takeBoard(ctx, boardID, g.ID)
	})
	if err != nil {
		return GameView{}, err
	}
	return s.Game(ctx, acc, g.ID)
}

// CreateTournament creates a tournament in its lobby, like CreateGame.
func (s *Service) CreateTournament(ctx context.Context, acc account.Account, t tournament.Tournament, playerID int64, boardID *int64) (TournamentView, error) {
	t.ID, t.StartedAt, t.CompletedAt, t.AbortedAt = 0, nil, nil, nil
	t.Settings = t.Settings.WithDefaults()
	playerID = firstPlayer(acc, playerID)
	t.Participants = []tournament.Participant{{PlayerID: playerID, Status: game.InvitationAccepted, BoardID: boardID}}
	if err := t.Validate(); err != nil {
		return TournamentView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOwnPlayers(ctx, acc, []int64{playerID}); err != nil {
		return TournamentView{}, err
	}
	seats := []game.Participant{{PlayerID: playerID, BoardID: boardID}}
	if err := s.takeSeats(ctx, acc, seat{lobby: t.Lobby}, seats, 0, func() error { return s.store.CreateTournament(ctx, &t) }); err != nil {
		return TournamentView{}, err
	}
	return s.tournamentView(ctx, acc, t)
}

// firstPlayer is the player a new game or tournament begins with: the one chosen, else the
// account's own.
func firstPlayer(acc account.Account, playerID int64) int64 {
	if playerID == 0 {
		return acc.PlayerID
	}
	return playerID
}

// AddPlayer seats a player of the account (its own or a guest) in a lobby it takes part
// in, on the board its other players there throw on (the first of them); SetSeatBoard
// changes it.
func (s *Service) AddPlayer(ctx context.Context, acc account.Account, ref LobbyRef, playerID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, pt, err := s.lobbyFor(ctx, acc, ref)
	if err != nil {
		return err
	}
	if err := s.checkOwnPlayers(ctx, acc, []int64{playerID}); err != nil {
		return err
	}
	if slices.Contains(playerIDs(l.seats), playerID) {
		return fmt.Errorf("%w: the player is in already", ErrInvalid)
	}
	if l.Full(len(l.seats)) && l.Open() {
		return ErrNoSeat
	}
	seat := game.Participant{PlayerID: playerID, Status: game.InvitationAccepted}
	for _, p := range l.seats {
		if slices.Contains(pt.mine, p.PlayerID) && p.Status == game.InvitationAccepted {
			seat.BoardID = p.BoardID
			break
		}
	}
	return s.takeSeats(ctx, acc, l.seat(), []game.Participant{seat}, 0, func() error {
		return s.addSeat(ctx, l, seat)
	})
}

// RemovePlayer takes a player of the account out of a lobby; without any of its players
// left it is as if the account left (LeaveGame, LeaveTournament), and a lobby nobody is in
// anymore is deleted.
func (s *Service) RemovePlayer(ctx context.Context, acc account.Account, ref LobbyRef, playerID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, pt, err := s.lobbyFor(ctx, acc, ref)
	if err != nil {
		return err
	}
	if !slices.Contains(pt.mine, playerID) || l.pending(playerID) {
		return ErrNotFound // an invitation is declined or withdrawn instead
	}
	return s.removeSeat(ctx, l, playerID)
}

// Invite invites a friend of the account to a lobby it plays in; the friend's player
// takes the seat once they accept (Accept).
func (s *Service) Invite(ctx context.Context, acc account.Account, ref LobbyRef, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, pt, err := s.lobbyFor(ctx, acc, ref)
	if err != nil {
		return err
	}
	if !pt.canPlay {
		return ErrForbidden
	}
	invited, err := s.invitedPlayers(ctx, acc, []string{username})
	switch {
	case err != nil:
		return err
	case len(invited) == 0:
		return fmt.Errorf("%w: no username", ErrInvalid)
	case slices.Contains(playerIDs(l.seats), invited[0]):
		return fmt.Errorf("%w: the player is in already", ErrInvalid)
	case l.Open() && l.Full(len(l.seats)):
		return ErrNoSeat
	}
	return s.addSeat(ctx, l, game.Participant{PlayerID: invited[0], Status: game.InvitationPending})
}

// Accept accepts the invitation of the account's own player to a lobby, on the board if
// one is given (one of the account's, or one other players throw on).
func (s *Service) Accept(ctx context.Context, acc account.Account, ref LobbyRef, boardID *int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, pt, err := s.lobbyFor(ctx, acc, ref)
	if err != nil {
		return err
	}
	if !pt.invited {
		return ErrNotFound
	}
	seat := game.Participant{PlayerID: acc.PlayerID, BoardID: boardID}
	return s.takeSeats(ctx, acc, l.seat(), []game.Participant{seat}, 0, func() error {
		if ref.Tournament {
			err = s.store.AcceptTournamentInvitation(ctx, ref.ID, acc.PlayerID, boardID)
		} else if err = s.store.AcceptInvitation(ctx, ref.ID, acc.PlayerID, boardID); err == nil {
			err = s.takeBoard(ctx, boardID, ref.ID)
		}
		if err != nil {
			return err
		}
		return s.seatsChanged(ctx, ref)
	})
}

// Decline declines the invitation of the account's own player to a lobby.
func (s *Service) Decline(ctx context.Context, acc account.Account, ref LobbyRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, pt, err := s.lobbyFor(ctx, acc, ref)
	if err != nil {
		return err
	}
	if !pt.invited {
		return ErrNotFound
	}
	return s.removeSeat(ctx, l, acc.PlayerID)
}

// Withdraw lets a player of a lobby take back an invitation nobody answered yet.
func (s *Service) Withdraw(ctx context.Context, acc account.Account, ref LobbyRef, playerID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, pt, err := s.lobbyFor(ctx, acc, ref)
	if err != nil {
		return err
	}
	if !pt.canPlay {
		return ErrForbidden
	}
	if !l.pending(playerID) {
		return fmt.Errorf("%w: player %d has no pending invitation", ErrInvalid, playerID)
	}
	return s.removeSeat(ctx, l, playerID)
}

// Join takes a free seat in a game or tournament open for strangers with the account's own
// player, on the board if one is given (one of the account's, or one other players throw
// on). Those that aren't open stay hidden (ErrNotFound); whoever takes the last seat
// starts it.
func (s *Service) Join(ctx context.Context, acc account.Account, ref LobbyRef, boardID *int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, err := s.loadLobby(ctx, ref)
	if err != nil {
		return err
	}
	switch {
	case !l.Open():
		return ErrNotFound
	case slices.Contains(playerIDs(l.seats), acc.PlayerID):
		return fmt.Errorf("%w: you are in already", ErrInvalid)
	case l.started || !offered(l.Lobby, len(l.seats), l.aborted):
		return ErrNoSeat
	}
	seat := game.Participant{PlayerID: acc.PlayerID, Status: game.InvitationAccepted, BoardID: boardID}
	return s.takeSeats(ctx, acc, l.seat(), []game.Participant{seat}, 0, func() error {
		return s.addSeat(ctx, l, seat)
	})
}

// SetSeatBoard puts a player of the account in a lobby on a board, or takes them off theirs
// with nil, like UseBoard does in a running game.
func (s *Service) SetSeatBoard(ctx context.Context, acc account.Account, ref LobbyRef, playerID int64, boardID *int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, pt, err := s.lobbyFor(ctx, acc, ref)
	if err != nil {
		return err
	}
	if !slices.Contains(pt.mine, playerID) || l.pending(playerID) {
		return ErrNotFound
	}
	if err := s.checkSeatBoard(ctx, acc, l.seat(), boardID); err != nil {
		return err
	}
	now := s.now()
	if ref.Tournament {
		err = s.store.SetTournamentParticipantBoard(ctx, ref.ID, playerID, boardID, now)
	} else if err = s.store.SetParticipantBoard(ctx, ref.ID, playerID, boardID, now); err == nil {
		err = s.takeBoard(ctx, boardID, ref.ID)
	}
	if err != nil {
		return err
	}
	s.notifyLobby(ref)
	return nil
}

// Start starts a game or tournament from its lobby: a player of it may, once no invitation
// is pending and no seat is free (ErrNotReady); a tournament needs enough players
// (tournament.ErrPlayerCount). The throwing order, or the draw, is drawn then.
func (s *Service) Start(ctx context.Context, acc account.Account, ref LobbyRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, pt, err := s.lobbyFor(ctx, acc, ref)
	if err != nil {
		return err
	}
	if !pt.canPlay {
		return ErrForbidden
	}
	if !l.CanStart(l.statuses()) {
		return ErrNotReady
	}
	if err := s.start(ctx, ref); err != nil {
		return err
	}
	s.notifyLobby(ref)
	return nil
}

// addSeat stores a new seat of the lobby, takes its board for a game, and starts an open
// one with its last seat. The caller holds s.mu.
func (s *Service) addSeat(ctx context.Context, l lobby, p game.Participant) error {
	var err error
	if l.ref.Tournament {
		err = s.store.AddTournamentParticipant(ctx, l.ref.ID, tournament.Participant{PlayerID: p.PlayerID, Status: p.Status, BoardID: p.BoardID})
	} else if err = s.store.AddParticipant(ctx, l.ref.ID, p); err == nil && p.Status == game.InvitationAccepted {
		err = s.takeBoard(ctx, p.BoardID, l.ref.ID)
	}
	if err != nil {
		return err
	}
	return s.seatsChanged(ctx, l.ref)
}

// removeSeat takes a player out of the lobby; one nobody is in anymore is deleted. The
// caller holds s.mu.
func (s *Service) removeSeat(ctx context.Context, l lobby, playerID int64) error {
	var err error
	if l.ref.Tournament {
		err = s.store.RemoveTournamentParticipant(ctx, l.ref.ID, playerID)
	} else {
		err = s.store.RemoveParticipant(ctx, l.ref.ID, playerID)
	}
	if err != nil {
		return err
	}
	return s.seatsChanged(ctx, l.ref)
}

// seatsChanged starts an open game or tournament whose last seat was just taken (one
// that isn't open waits for a player to start it) and tells whoever looks at it. The
// caller holds s.mu.
func (s *Service) seatsChanged(ctx context.Context, ref LobbyRef) error {
	l, err := s.loadLobby(ctx, ref)
	if errors.Is(err, store.ErrNotFound) { // nobody is left in it
		s.notifyLobby(ref)
		return nil
	}
	if err != nil {
		return err
	}
	if l.Open() && !l.started && l.CanStart(l.statuses()) {
		if err := s.start(ctx, ref); err != nil {
			return err
		}
	}
	s.notifyLobby(ref)
	return nil
}

// start draws the order of the players and starts the game or tournament. The caller
// holds s.mu.
func (s *Service) start(ctx context.Context, ref LobbyRef) error {
	if ref.Tournament {
		t, err := s.store.GetTournament(ctx, ref.ID)
		if err != nil {
			return err
		}
		if err := t.CheckPlayerCount(); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalid, err)
		}
		s.shuffle(len(t.Participants), func(i, j int) { t.Participants[i], t.Participants[j] = t.Participants[j], t.Participants[i] })
		now := s.now()
		t.StartedAt = &now
		return s.store.StartTournament(ctx, &t)
	}
	return s.startGame(ctx, ref.ID)
}

// startGame starts a game: the throwing order is drawn (not for a tournament's match,
// whose order its schedule gives), the first turn's time begins and every replica sets
// its timer. The caller holds s.mu.
func (s *Service) startGame(ctx context.Context, gameID int64) error {
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		return err
	}
	if g.TournamentID == nil {
		s.shuffle(len(g.Participants), func(i, j int) { g.Participants[i], g.Participants[j] = g.Participants[j], g.Participants[i] })
	}
	now := s.now()
	g.StartedAt = &now
	if err := s.store.StartGame(ctx, &g); err != nil {
		return err
	}
	s.publish(turnKeys(g)...)
	return nil
}

// notifyLobby tells whoever looks at the game or tournament that it changed.
func (s *Service) notifyLobby(ref LobbyRef) {
	if ref.Tournament {
		s.notifyTournament(ref.ID)
	} else {
		s.notify(ref.ID)
	}
}
