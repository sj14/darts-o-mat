package view

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/service"
)

// progress is the part of the game state that is the same for all modes.
type progress struct {
	CurrentPlayerID *int64
	Round           int
	DartsInTurn     int
	GameOver        bool
	WinnerIDs       []int64
	// WaitingForTakeout: the turn is over, the darts still have to be pulled out.
	WaitingForTakeout bool
}

func progressOf(v service.GameView) progress {
	switch st := v.State.(type) {
	case x01.State:
		return progress{st.CurrentPlayerID, st.Round, st.DartsInTurn, st.GameOver, st.WinnerIDs, st.WaitingForTakeout}
	case aroundtheclock.State:
		return progress{st.CurrentPlayerID, st.Round, st.DartsInTurn, st.GameOver, st.WinnerIDs, st.WaitingForTakeout}
	}
	return progress{}
}

func (p progress) isCurrent(playerID int64) bool {
	return p.CurrentPlayerID != nil && *p.CurrentPlayerID == playerID
}

func (p progress) isWinner(playerID int64) bool {
	for _, id := range p.WinnerIDs {
		if id == playerID {
			return true
		}
	}
	return false
}

func playerName(v service.GameView, id int64) string {
	for _, p := range v.Players {
		if p.ID == id {
			return p.Name
		}
	}
	return "?"
}

func playerNames(v service.GameView, ids []int64) string {
	var names []string
	for _, id := range ids {
		names = append(names, playerName(v, id))
	}
	return strings.Join(names, ", ")
}

// turnDarts returns the darts already thrown in the current turn.
func turnDarts(v service.GameView, p progress) []throw.Dart {
	if p.DartsInTurn > len(v.Darts) {
		return nil
	}
	return v.Darts[len(v.Darts)-p.DartsInTurn:]
}

func dartLabel(d throw.Dart) string {
	switch {
	case d.Miss:
		return "Daneben"
	case d.Number == throw.Bull && d.Multiplier == 2:
		return "Bullseye"
	case d.Number == throw.Bull:
		return "Bull"
	case d.Multiplier == 2:
		return fmt.Sprintf("D%d", d.Number)
	case d.Multiplier == 3:
		return fmt.Sprintf("T%d", d.Number)
	}
	return strconv.Itoa(int(d.Number))
}

func targetLabel(target uint8) string {
	switch target {
	case 0:
		return "✓"
	case throw.Bull:
		return "Bull"
	}
	return strconv.Itoa(int(target))
}

func outModeLabel(m x01.OutMode) string {
	switch m {
	case x01.OutModeSingle:
		return "Single Out"
	case x01.OutModeDouble:
		return "Double Out"
	case x01.OutModeMaster:
		return "Master Out"
	}
	return ""
}

func modeLabel(g game.Game) string {
	switch g.Mode {
	case game.ModeX01:
		return fmt.Sprintf("%d %s", g.X01.StartScore, outModeLabel(g.X01.OutMode))
	case game.ModeAroundTheClock:
		label := "Around the Clock"
		if g.AroundTheClock.Bull {
			label += " mit Bull"
		}
		if g.AroundTheClock.MultiplierSkips {
			label += ", Doppel/Triple springen"
		}
		return label
	}
	return string(g.Mode)
}

// currentTarget returns the target of the current player in Around the Clock, 0 otherwise.
func currentTarget(v service.GameView) uint8 {
	st, ok := v.State.(aroundtheclock.State)
	if !ok || st.CurrentPlayerID == nil {
		return 0
	}
	for _, p := range st.Players {
		if p.PlayerID == *st.CurrentPlayerID {
			return p.Target
		}
	}
	return 0
}

func gameURL(id int64, suffix string) string {
	return fmt.Sprintf("/games/%d%s", id, suffix)
}

// decimal formats a number with one decimal place and a German decimal comma.
func decimal(f float64) string {
	return strings.Replace(strconv.FormatFloat(f, 'f', 1, 64), ".", ",", 1)
}

// percent formats a ratio from 0 to 1 as a whole percentage.
func percent(ratio float64) string {
	return strconv.Itoa(int(math.Round(ratio*100))) + " %"
}

// orDash returns value if ok, otherwise a dash for "no value yet".
func orDash(ok bool, value string) string {
	if !ok {
		return "–"
	}
	return value
}

// nextPlayerName returns who throws after the current player.
func nextPlayerName(v service.GameView, p progress) string {
	if p.CurrentPlayerID == nil {
		return ""
	}
	for i, id := range v.Game.PlayerIDs {
		if id == *p.CurrentPlayerID {
			return playerName(v, v.Game.PlayerIDs[(i+1)%len(v.Game.PlayerIDs)])
		}
	}
	return ""
}

// dartEditURL is where a dart is corrected; GET shows the keypad, POST saves it.
func dartEditURL(v service.GameView, d throw.Dart) string {
	return gameURL(v.Game.ID, "/darts/"+strconv.FormatInt(d.ID, 10))
}

// dartThrower returns the name of the player who threw a dart of the game.
func dartThrower(v service.GameView, dartID int64) string {
	throwers := game.Throwers(v.State)
	for i, d := range v.Darts {
		if d.ID == dartID && i < len(throwers) {
			return playerName(v, throwers[i])
		}
	}
	return "?"
}
