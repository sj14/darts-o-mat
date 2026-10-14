package view

import (
	"fmt"
	"math"
	"slices"
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
	turn.State
	Aborted bool
}

func progressOf(v service.GameView) progress {
	p := progress{State: v.Common()}
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

func targetLabel(t Texts, target uint8) string {
	switch target {
	case 0:
		return "✓"
	case throw.BullNumber:
		return t.Bull()
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

func modeLabel(t Texts, g game.Game) string {
	switch {
	case g.Sets > 1:
		return modeName(t, g) + " · " + t.FirstToSets(g.Sets, g.Legs)
	case g.Legs > 1:
		return modeName(t, g) + " · First to " + strconv.Itoa(g.Legs)
	}
	return modeName(t, g)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func modeName(t Texts, g game.Game) string {
	switch g.Mode {
	case game.ModeX01:
		return fmt.Sprintf("%d %s", g.X01.StartScore, outModeLabel(g.X01.OutMode))
	case game.ModeAroundTheClock:
		label := t.AroundTheClock()
		if g.AroundTheClock.Bull {
			label += t.WithBull()
		}
		if g.AroundTheClock.MultiplierSkips {
			label += t.MultiplierSkips()
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

// orDash returns value if ok, otherwise a dash for "no value yet".
func orDash(ok bool, value string) string {
	if !ok {
		return "–"
	}
	return value
}

// playerLine is a player's main value (rest, target) and second key figure, as the
// player list shows them.
type playerLine struct {
	ID                                int64
	Sets, Legs                        int // won; legs in the current set
	Value, Caption, Stat, StatCaption string
}

// playerLines returns the players in the order they throw from now on: the current
// player first (the winner once the game is over), then the others.
func playerLines(t Texts, v service.GameView, p progress) []playerLine {
	var lines []playerLine
	switch st := v.State.(type) {
	case x01.State:
		for _, ps := range st.Players {
			lines = append(lines, playerLine{ps.PlayerID, ps.Sets, ps.SetLegs, strconv.FormatUint(ps.Remaining, 10), t.Remaining(), orDash(ps.Stats.Darts > 0, t.Decimal(ps.Stats.Average)), t.Average3()})
		}
	case aroundtheclock.State:
		for _, ps := range st.Players {
			lines = append(lines, playerLine{ps.PlayerID, ps.Sets, ps.SetLegs, targetLabel(t, ps.Target), t.Target(), orDash(ps.Stats.Darts > 0, t.Percent(ps.Stats.HitRate)), t.HitRate()})
		}
	}
	first := p.CurrentPlayerID
	if first == nil {
		first = p.WinnerID
	}
	if first != nil {
		if i := slices.IndexFunc(lines, func(l playerLine) bool { return l.ID == *first }); i > 0 {
			lines = slices.Concat(lines[i:], lines[:i])
		}
	}
	return lines
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
	throwers := v.Common().Throwers
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
	case d.Segment == throw.Miss:
		return "miss"
	case hitTarget(v, d):
		return "hit"
	}
	return ""
}

// dartLabel is how a dart is shown: 20, D20, T20, Bull, Bullseye, M7 (a miss beside the
// 7), or "Miss" when unknown where.
func dartLabel(t Texts, d throw.Dart) string {
	switch {
	case d.Segment == throw.Miss && d.Number == 0:
		return t.Miss()
	case d.Segment == throw.Bullseye:
		return t.Bullseye()
	case d.Segment == throw.Bull:
		return t.Bull()
	}
	return d.String()
}

// dartTileClasses style a dart's large tile; the dart just thrown flies in.
func dartTileClasses(t Texts, v service.GameView, d throw.Dart) string {
	classes := "flex h-20 items-center justify-center rounded-2xl font-display font-extrabold uppercase shadow-lg transition sm:h-24 lg:h-20 " + dartTileColors(v, d)
	if len([]rune(dartLabel(t, d))) > 4 {
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
	hit, _ := targetResult(v, d)
	return hit
}

// targetResult reports for around the clock whether a dart hit the player's target;
// counted is false in other modes and for darts outside the turns (bull-off).
func targetResult(v service.GameView, d throw.Dart) (hit, counted bool) {
	st, ok := v.State.(aroundtheclock.State)
	if !ok {
		return false, false
	}
	for _, t := range st.Turns {
		for i, td := range t.Darts {
			if td.ID == d.ID {
				return t.Hits[i], true
			}
		}
	}
	return false, false
}

// soundKind returns which sound the last entry of the game's log makes (see
// static/sounds.js); soundID tells the page whether that entry is new.
func soundKind(v service.GameView) string {
	if len(v.Darts) == 0 {
		return ""
	}
	last, c := v.Darts[len(v.Darts)-1], v.Common()
	if event := latestTurnEvent(v); event != "" && !c.GameOver {
		return event
	}
	hit, counted := targetResult(v, last)
	switch {
	case last.Takeout:
		return "takeout"
	case c.GameOver || c.LegWinner != nil:
		return "win"
	case last.Segment == throw.Miss || counted && !hit: // in around the clock, every dart off the target misses
		return "miss"
	case hit:
		return "hit"
	case last.Segment.IsBull():
		return "bull"
	case last.Segment == throw.Triple:
		return "triple"
	case last.Segment == throw.Double:
		return "double"
	}
	return "dart"
}

// soundID is the id of the last entry of the game's log, 0 without one.
func soundID(v service.GameView) string {
	if len(v.Darts) == 0 {
		return "0"
	}
	return strconv.FormatInt(v.Darts[len(v.Darts)-1].ID, 10)
}

// checkoutSuggestion is how the current player can finish with the darts left in the
// turn (X01 only), nil if that isn't possible.
func checkoutSuggestion(v service.GameView, p progress) []throw.Dart {
	st, ok := v.State.(x01.State)
	if !ok || !p.running() || p.WaitingForTakeout || p.BullingOff || p.CurrentPlayerID == nil {
		return nil
	}
	for _, ps := range st.Players {
		if ps.PlayerID == *p.CurrentPlayerID {
			return v.Game.X01.OutMode.Checkout(ps.Remaining, turn.DartsPerTurn-p.DartsInTurn)
		}
	}
	return nil
}

// bullOffDecided returns who won the bull-off, if the dart just thrown decided it.
func bullOffDecided(v service.GameView, p progress) (int64, bool) {
	darts := p.BullOff
	if p.BullingOff || len(darts) == 0 || darts[len(darts)-1].Entry != len(v.Darts)-1 || p.CurrentPlayerID == nil {
		return 0, false
	}
	return *p.CurrentPlayerID, true
}

// bullOffDistance describes how close a bull-off dart came: in mm if the cameras saw it.
func bullOffDistance(t Texts, d throw.Dart) string {
	if d.X == nil || d.Y == nil {
		return ""
	}
	return t.Decimal(math.Hypot(*d.X, *d.Y)) + " mm"
}

// hasTurns reports whether the game has darts beyond the bull-off.
func hasTurns(v service.GameView) bool {
	return len(v.Darts) > takeouts(v)+len(v.Common().BullOff)
}

// takeouts counts the takeouts in the game's log.
func takeouts(v service.GameView) int {
	n := 0
	for _, d := range v.Darts {
		if d.Takeout {
			n++
		}
	}
	return n
}

// lastUsedBoard is the ID of the board a game was last started on, 0 if none was used yet.
func lastUsedBoard(boards []service.BoardView) int64 {
	var last service.BoardView
	for _, b := range boards {
		if b.LastUsedAt != nil && (last.LastUsedAt == nil || b.LastUsedAt.After(*last.LastUsedAt)) {
			last = b
		}
	}
	return last.ID
}
