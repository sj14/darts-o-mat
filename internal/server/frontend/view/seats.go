package view

import (
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

// lobbyModel is the lobby of a game or a tournament as lobbyPanel shows it.
type lobbyModel struct {
	base string // the page of the game or tournament, the lobby's actions are below it
	game.Lobby
	seats      []lobbySeat
	tournament bool
	invited    bool // the viewer's own player is invited
	canPlay    bool // the viewer has a player in it: may add, invite, start
	canStart   bool // no invitation is pending and no seat is free
	boards     []board.Board
	choices    *service.LobbyChoices
	// leaveConfirm asks before the viewer's last player is taken out, which is leaving:
	// the same question as the button to leave.
	leaveConfirm string
}

// lobbySeat is a player in the lobby, or invited to it.
type lobbySeat struct {
	player  player.Player
	pending bool
	mine    bool   // one of the viewer's players
	boardID *int64 // the board they throw on, nil for none
	board   string // its name, if the viewer knows it
}

func gameLobby(t Texts, v service.GameView) lobbyModel {
	m := lobbyModel{base: gameURL(v.Game.ID, ""), Lobby: v.Game.Lobby, invited: v.Invited, canPlay: v.CanThrow, canStart: v.CanStart, boards: boardsOfChoices(v.Boards), choices: v.Choices, leaveConfirm: leaveConfirm(t, v)}
	for _, p := range v.Players {
		m.seats = append(m.seats, lobbySeat{player: p, pending: v.Pending(p.ID), mine: v.IsMine(p.ID), boardID: v.BoardOf(p.ID), board: v.BoardNames[p.ID]})
	}
	return m
}

func tournamentLobby(t Texts, v service.TournamentView) lobbyModel {
	m := lobbyModel{base: tournamentURL(v.Tournament.ID, ""), Lobby: v.Tournament.Lobby, tournament: true, invited: v.Invited, canPlay: v.CanPlay, canStart: v.CanStart, boards: boardsOfChoices(v.Boards), choices: v.Choices, leaveConfirm: t.LeaveTournamentConfirm()}
	for _, p := range v.Players {
		tp, _ := v.Tournament.Participant(p.ID)
		s := lobbySeat{player: p, pending: v.Pending(p.ID), boardID: tp.BoardID}
		for _, id := range v.Mine {
			s.mine = s.mine || id == p.ID
		}
		for _, b := range v.Boards {
			if tp.BoardID != nil && b.ID == *tp.BoardID {
				s.board = b.Name
			}
		}
		m.seats = append(m.seats, s)
	}
	return m
}

// url is the URL of an action of the lobby.
func (m lobbyModel) url(suffix string) string {
	return m.base + suffix
}

// lastOfMine reports whether the seat is the viewer's only player in the lobby: taking it
// out leaves.
func (m lobbyModel) lastOfMine(s lobbySeat) bool {
	for _, o := range m.seats {
		if o.mine && !o.pending && o.player.ID != s.player.ID {
			return false
		}
	}
	return true
}

// startable reports whether a player may start it now: everyone answered, and a
// tournament has enough players. An open one starts by itself with its last seat.
func (m lobbyModel) startable() bool {
	return m.canStart && !m.Open() && (!m.tournament || len(m.seats) >= tournament.MinPlayers)
}
