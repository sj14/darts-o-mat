package game

import (
	"errors"
	"log"
	"math/rand/v2"

	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"gorm.io/gorm"
)

type PlayerState struct {
	player.Player
	Board   board.Board
	Counter uint64
	Throws  []throw.Dart
}

type OutMode uint8

const (
	_                     = iota
	OutModeSingle OutMode = iota
	OutModeDouble OutMode = iota
	OutModeTriple OutMode = iota
)

type GameX01 struct {
	gorm.Model
	GameID        uint64
	HitCounter    uint64
	CurrentRound  uint8
	CurrentPlayer uint
	Players       []*PlayerState
	OutMode       OutMode
}

func NewX01(hitCounter uint64, players []player.Player) (GameX01, error) {
	if len(players) == 0 {
		return GameX01{}, errors.New("not enough players")
	}

	g := GameX01{
		GameID:        rand.Uint64(),
		HitCounter:    hitCounter,
		CurrentPlayer: players[0].ID,
	}

	for _, p := range players {
		g.Players = append(g.Players, &PlayerState{Player: p})
	}

	return g, nil
}

func (g *GameX01) NextPlayer() {
	for i, player := range g.Players {

		if player.ID == g.CurrentPlayer {

			if i+1 == len(g.Players) {
				g.CurrentPlayer = g.Players[0].ID
			} else {
				g.CurrentPlayer = g.Players[i+1].ID
			}
			return
		}
	}
}

func (g *GameX01) currentPlayer() *PlayerState {
	for _, p := range g.Players {
		if p.ID == g.CurrentPlayer {
			return p
		}
	}
	panic("not found")
}

func (g *GameX01) RegisterDart(throw throw.Dart) {
	player := g.currentPlayer()
	player.Throws = append(player.Throws, throw)

	defer func() {
		if len(player.Throws)%3 == 0 {
			g.NextPlayer()
			g.CurrentRound++
		}
	}()

	value, miss := throw.Value()

	if miss {
		return
	}

	possibleNewPlayerCounter := player.Counter + uint64(value)
	if possibleNewPlayerCounter <= g.HitCounter {

		if possibleNewPlayerCounter != g.HitCounter && g.HitCounter-possibleNewPlayerCounter < uint64(g.OutMode) {
			// nicht möglich double/triple out zu machen
			log.Println("busted")
			return
		}

		player.Counter = possibleNewPlayerCounter
	} else {
		log.Println("busted")
	}

	// TODO: kann der nächste spieler nahziehen?
	if player.Counter == g.HitCounter {
		// Winner
		log.Printf("player %v WINS\n", player)
	}
}
