package view

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/service"
)

// progress is the part of the game state that is the same for all modes.
type progress struct {
	CurrentPlayerID *int64
	Round           int
	DartsInTurn     int
	Leg             int
	GameOver        bool
	WinnerID        *int64
	// WaitingForTakeout: the turn is over, the darts still have to be pulled out.
	WaitingForTakeout bool
	// LegOver: the turn waiting for the takeout won a leg.
	LegOver bool
	Aborted bool
}

func progressOf(v service.GameView) progress {
	var p progress
	switch st := v.State.(type) {
	case x01.State:
		p = progress{st.CurrentPlayerID, st.Round, st.DartsInTurn, st.Leg, st.GameOver, st.WinnerID, st.WaitingForTakeout, st.LegOver, false}
	case aroundtheclock.State:
		p = progress{st.CurrentPlayerID, st.Round, st.DartsInTurn, st.Leg, st.GameOver, st.WinnerID, st.WaitingForTakeout, st.LegOver, false}
	}
	if v.Game.Aborted() {
		// Nobody is up anymore.
		p.CurrentPlayerID, p.WaitingForTakeout, p.Aborted = nil, false, true
	}
	return p
}

// running reports whether the game still takes darts.
func (p progress) running() bool {
	return !p.GameOver && !p.Aborted
}

func (p progress) isCurrent(playerID int64) bool {
	return p.CurrentPlayerID != nil && *p.CurrentPlayerID == playerID
}

func (p progress) isWinner(playerID int64) bool {
	return p.WinnerID != nil && *p.WinnerID == playerID
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
	if g.Legs > 1 {
		return modeName(g) + " · First to " + strconv.Itoa(g.Legs)
	}
	return modeName(g)
}

func modeName(g game.Game) string {
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

// countTrue counts the set flags, e.g. the hits of a turn.
func countTrue(flags []bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

// dartClass marks a dart for coloring: "miss" for misses, "hit" for darts that hit the
// target in around the clock, empty otherwise.
func dartClass(v service.GameView, d throw.Dart) string {
	switch {
	case d.Miss:
		return "miss"
	case hitTarget(v, d):
		return "hit"
	}
	return ""
}

// dartTileClasses style a dart's large tile; the dart just thrown flies in.
func dartTileClasses(v service.GameView, d throw.Dart) string {
	classes := "flex h-20 items-center justify-center rounded-2xl font-display font-extrabold uppercase shadow-lg transition sm:h-24 " + dartTileColors(v, d)
	if len(dartLabel(d)) > 4 {
		classes += " text-2xl sm:text-3xl"
	} else {
		classes += " text-5xl"
	}
	if isLatest(v, d) {
		classes += " animate-dart-in"
	}
	return classes
}

// dartTileColors are the colors of a dart's large tile: like dartClass, cream for other darts.
func dartTileColors(v service.GameView, d throw.Dart) string {
	switch dartClass(v, d) {
	case "miss":
		return "bg-board-red text-white shadow-board-red/30"
	case "hit":
		return "bg-board-green text-white shadow-board-green/30"
	}
	return "bg-board-cream text-pub-950 shadow-black/30"
}

// dartChipColors are the colors of a dart in the history.
func dartChipColors(v service.GameView, d throw.Dart) string {
	switch dartClass(v, d) {
	case "miss":
		return "bg-board-red/20 text-red-300"
	case "hit":
		return "bg-board-green/25 text-emerald-300"
	}
	return "bg-white/10 text-board-cream"
}

// isLatest reports whether d is the last entry of the game's log, i.e. just thrown.
func isLatest(v service.GameView, d throw.Dart) bool {
	return len(v.Darts) > 0 && v.Darts[len(v.Darts)-1].ID == d.ID && !d.Takeout
}

// latestTurnEvent is "180" or "bust" if the dart just thrown completed such a turn in X01,
// empty otherwise.
func latestTurnEvent(v service.GameView) string {
	st, ok := v.State.(x01.State)
	if !ok || len(st.Turns) == 0 || len(v.Darts) == 0 || v.Darts[len(v.Darts)-1].Takeout {
		return ""
	}
	t := st.Turns[len(st.Turns)-1]
	if len(t.Darts) == 0 || t.Darts[len(t.Darts)-1].ID != v.Darts[len(v.Darts)-1].ID || !t.Complete {
		return ""
	}
	switch {
	case t.Bust:
		return "bust"
	case t.Points == 180:
		return "180"
	}
	return ""
}

// hitTarget reports whether a dart hit the player's target in around the clock.
func hitTarget(v service.GameView, d throw.Dart) bool {
	st, ok := v.State.(aroundtheclock.State)
	if !ok {
		return false
	}
	for _, t := range st.Turns {
		for i, td := range t.Darts {
			if td.ID == d.ID {
				return t.Hits[i]
			}
		}
	}
	return false
}

// sound returns which sound the last entry of the game's log makes (see static/sounds.js)
// and the entry's id, which tells the page whether it is new.
func sound(v service.GameView) (kind, id string) {
	if len(v.Darts) == 0 {
		return "", "0"
	}
	last := v.Darts[len(v.Darts)-1]
	id = strconv.FormatInt(last.ID, 10)
	switch {
	case last.Takeout:
		return "takeout", id
	case progressOf(v).GameOver:
		return "win", id
	}
	if _, won := legWon(v, progressOf(v)); won {
		return "win", id
	}
	if event := latestTurnEvent(v); event != "" {
		return event, id
	}
	switch {
	case last.Miss:
		return "miss", id
	case hitTarget(v, last):
		return "hit", id
	case last.Number == throw.Bull:
		return "bull", id
	case last.Multiplier == 3:
		return "triple", id
	case last.Multiplier == 2:
		return "double", id
	}
	return "dart", id
}

func soundKind(v service.GameView) string {
	kind, _ := sound(v)
	return kind
}

func soundID(v service.GameView) string {
	_, id := sound(v)
	return id
}

// checkoutSuggestion is how the current player can finish with the darts left in the
// turn (X01 only), nil if that isn't possible.
func checkoutSuggestion(v service.GameView, p progress) []throw.Dart {
	st, ok := v.State.(x01.State)
	if !ok || !p.running() || p.WaitingForTakeout || p.CurrentPlayerID == nil {
		return nil
	}
	for _, ps := range st.Players {
		if ps.PlayerID == *p.CurrentPlayerID {
			return v.Game.X01.OutMode.Checkout(ps.Remaining, turn.DartsPerTurn-p.DartsInTurn)
		}
	}
	return nil
}

// legWon reports whether the dart just thrown won a leg without ending the game, and
// whose turn it was.
func legWon(v service.GameView, p progress) (int64, bool) {
	if p.GameOver || len(v.Darts) == 0 || v.Darts[len(v.Darts)-1].Takeout {
		return 0, false
	}
	var leg, lastLeg int
	var player int64
	switch st := v.State.(type) {
	case x01.State:
		if len(st.Turns) == 0 || !st.Turns[len(st.Turns)-1].Checkout {
			return 0, false
		}
		t := st.Turns[len(st.Turns)-1]
		leg, lastLeg, player = st.Leg, t.Leg, t.PlayerID
	case aroundtheclock.State:
		if len(st.Turns) == 0 || st.Turns[len(st.Turns)-1].Target != 0 {
			return 0, false
		}
		t := st.Turns[len(st.Turns)-1]
		leg, lastLeg, player = st.Leg, t.Leg, t.PlayerID
	}
	// Either the next leg already started, or it waits for the takeout.
	return player, leg > lastLeg || p.LegOver
}
