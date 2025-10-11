package x01

import (
	"errors"
	"log"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

type X01PlayerState struct {
	// gorm.Model
	ID      uint64
	Counter uint64
	Game    game.PlayerState
}

type OutMode uint8

const (
	_                     = iota
	OutModeSingle OutMode = iota
	OutModeDouble OutMode = iota
	OutModeTriple OutMode = iota
)

type GameX01 struct {
	ID            uint64
	HitCounter    uint64
	CurrentRound  uint8
	CurrentPlayer uint64
	Players       []*X01PlayerState
	OutMode       OutMode
}

func NewX01(hitCounter uint64, players []player.Player) (GameX01, error) {
	if len(players) == 0 {
		return GameX01{}, errors.New("not enough players")
	}

	g := GameX01{
		// ID:            rand.Uint64(),
		HitCounter:    hitCounter,
		CurrentPlayer: players[0].ID,
	}

	for _, p := range players {
		g.Players = append(g.Players, &X01PlayerState{Game: game.PlayerState{Player: p}})
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

func (g *GameX01) currentPlayer() *X01PlayerState {
	for _, p := range g.Players {
		if p.ID == g.CurrentPlayer {
			return p
		}
	}
	panic("not found")
}

func (g *GameX01) RegisterDart(throw throw.Dart) {
	player := g.currentPlayer()
	player.Game.Throws = append(player.Game.Throws, throw)

	defer func() {
		if len(player.Game.Throws)%3 == 0 {
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
