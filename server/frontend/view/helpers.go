package view

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/a-h/templ"

	"github.com/sj14/darts-o-mat/common/dartboard"
	"github.com/sj14/darts-o-mat/server/domain/board"
	"github.com/sj14/darts-o-mat/server/domain/game"
	"github.com/sj14/darts-o-mat/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/server/domain/player"
	"github.com/sj14/darts-o-mat/server/domain/stats"
	"github.com/sj14/darts-o-mat/server/domain/throw"
	"github.com/sj14/darts-o-mat/server/service"
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

func playerName(ctx context.Context, v service.GameView, id int64) string {
	for _, p := range v.Players {
		if p.ID == id {
			return displayName(ctx, p)
		}
	}
	return "?"
}

// displayName is a player's name as shown: "deleted player" once they were deleted.
func displayName(ctx context.Context, p player.Player) string {
	if p.Deleted() {
		return T(ctx).DeletedPlayer()
	}
	return p.Name
}

func playerNames(ctx context.Context, v service.GameView, ids []int64) string {
	var names []string
	for _, id := range ids {
		names = append(names, playerName(ctx, v, id))
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
	case dartboard.BullNumber:
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

// modeGoal is what a game is played to, "First to 3", or empty for a single leg.
func modeGoal(t Texts, g game.Settings) string {
	switch {
	case g.Sets > 1:
		return t.FirstToSets(g.Sets, g.Legs)
	case g.Legs > 1:
		return "First to " + strconv.Itoa(g.Legs)
	}
	return ""
}

func modeLabel(t Texts, g game.Settings) string {
	if goal := modeGoal(t, g); goal != "" {
		return modeName(t, g) + " · " + goal
	}
	return modeName(t, g)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// gameHeat is where a player's darts hit the board in this game, from the cameras.
func gameHeat(v service.GameView, playerID int64) stats.Heat {
	var points []dartboard.Point
	for _, d := range v.Darts {
		if d.PlayerID == playerID && d.IsDart() && d.X != nil && d.Y != nil {
			points = append(points, dartboard.Point{X: *d.X, Y: *d.Y})
		}
	}
	return stats.NewHeat(points)
}

func modeName(t Texts, g game.Settings) string {
	switch g.Mode {
	case game.ModeX01:
		r := g.Rules.(*x01.Settings)
		return fmt.Sprintf("%d %s", r.StartScore, outModeLabel(r.OutMode))
	case game.ModeAroundTheClock:
		r := g.Rules.(*aroundtheclock.Settings)
		label := t.AroundTheClock()
		if r.Bull {
			label += t.WithBull()
		}
		if r.MultiplierSkips {
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
	p, _ := st.Player(*st.CurrentPlayerID)
	return p.Target
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
	Left                              bool // left the game
}

// playerLines returns the players in the order they throw from now on: the current
// player first (the winner once the game is over), then the others.
func playerLines(t Texts, v service.GameView, p progress) []playerLine {
	var lines []playerLine
	switch st := v.State.(type) {
	case x01.State:
		for _, ps := range st.Players {
			lines = append(lines, playerLine{ps.PlayerID, ps.Sets, ps.SetLegs, strconv.FormatUint(ps.Remaining, 10), t.Remaining(), orDash(ps.Stats.Darts > 0, t.Decimal(ps.Stats.Average)), t.Average3(), ps.Left})
		}
	case aroundtheclock.State:
		for _, ps := range st.Players {
			lines = append(lines, playerLine{ps.PlayerID, ps.Sets, ps.SetLegs, targetLabel(t, ps.Target), t.Target(), orDash(ps.Stats.Darts > 0, t.Percent(ps.Stats.HitRate)), t.HitRate(), ps.Left})
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
	// Players who left come last (the first line is the current player or the winner).
	if len(lines) > 1 {
		slices.SortStableFunc(lines[1:], func(a, b playerLine) int { return boolInt(a.Left) - boolInt(b.Left) })
	}
	return lines
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nextPlayerName returns who throws after the current player, not counting those who left.
func nextPlayerName(ctx context.Context, v service.GameView, p progress) string {
	if p.CurrentPlayerID == nil {
		return ""
	}
	ids := v.Game.PlayerIDs()
	for i, id := range ids {
		if id == *p.CurrentPlayerID {
			for j := 1; j < len(ids); j++ {
				if next := ids[(i+j)%len(ids)]; !v.Common().Records[next].Left {
					return playerName(ctx, v, next)
				}
			}
		}
	}
	return ""
}

// dartEditURL is where a dart is corrected; GET shows the keypad, POST saves it.
func dartEditURL(v service.GameView, d throw.Dart) string {
	return gameURL(v.Game.ID, "/darts/"+strconv.FormatInt(d.ID, 10))
}

// dartThrower returns the name of the player who threw a dart of the game.
func dartThrower(ctx context.Context, v service.GameView, dartID int64) string {
	for _, d := range v.Darts {
		if d.ID == dartID {
			return playerName(ctx, v, d.PlayerID)
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
	case d.Segment == dartboard.Miss:
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
	case d.Segment == dartboard.Miss && d.Number == 0:
		return t.Miss()
	case d.Segment == dartboard.Bullseye:
		return t.Bullseye()
	case d.Segment == dartboard.Bull:
		return t.Bull()
	}
	return d.String()
}

// fitStyle sets what text-fit needs (see fitText): the label's length in characters
// (at least 3, so short labels don't grow beyond the others) and the largest size.
func fitStyle(label, maxSize string) templ.SafeCSS {
	return templ.SafeCSS("--fit-len:" + strconv.Itoa(max(utf8.RuneCountInString(label), 3)) + ";--fit-max:" + maxSize)
}

// dartTileClasses style a dart's large tile; the dart just thrown flies in.
func dartTileClasses(v service.GameView, d throw.Dart) string {
	classes := "@container flex h-14 items-center justify-center rounded-xl px-2 font-number font-extrabold uppercase shadow-lg transition sm:h-16 " + dartTileColors(v, d)
	if isLatest(v, d) {
		classes += " animate-dart-in"
	}
	return classes
}

// dartTileColors are the colors of a dart's large tile: like dartClass, cream for other darts.
func dartTileColors(v service.GameView, d throw.Dart) string {
	switch dartClass(v, d) {
	case "miss":
		return "bg-board-red text-board-cream shadow-board-red/30"
	case "hit":
		return "bg-board-green text-board-cream shadow-board-green/30"
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
	return len(v.Darts) > 0 && v.Darts[len(v.Darts)-1].ID == d.ID && d.IsDart()
}

// latestTurnEvent is "180" or "bust" if the dart just thrown completed such a turn in X01,
// empty otherwise.
func latestTurnEvent(v service.GameView) string {
	st, ok := v.State.(x01.State)
	if !ok || len(st.Turns) == 0 || len(v.Darts) == 0 || !v.Darts[len(v.Darts)-1].IsDart() {
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

// turnClock returns how many seconds of the time for the turn are gone and how many it
// has in total, if the turn is timed.
func turnClock(v service.GameView, p progress) (elapsed, total float64, ok bool) {
	deadline, ok := v.Game.TurnDeadline(v.Darts, v.Common())
	if !ok || !p.running() {
		return 0, 0, false
	}
	total = float64(v.Game.TurnSeconds)
	return min(max(total-time.Until(deadline).Seconds(), 0), total), total, true
}

// turnClockStyle makes the bar of the turn clock start where the clock is: its animation
// lasts the whole time and has run for the elapsed part already.
func turnClockStyle(elapsed, total float64) templ.SafeCSS {
	return templ.SafeCSS(fmt.Sprintf("animation-duration:%.1fs;animation-delay:-%.1fs", total, elapsed))
}

// soundKind returns which sound the last entry of the game's log makes (see
// static/sounds.js); soundID tells the page whether that entry is new.
func soundKind(v service.GameView) string {
	if len(v.Darts) == 0 {
		return ""
	}
	last, c := v.Darts[len(v.Darts)-1], v.Common()
	if last.Leave || last.Skip {
		return ""
	}
	if event := latestTurnEvent(v); event != "" && !c.GameOver {
		return event
	}
	hit, counted := targetResult(v, last)
	switch {
	case last.Takeout:
		return "takeout"
	case c.GameOver || c.LegWinner != nil:
		return "win"
	case last.Segment == dartboard.Miss || counted && !hit: // in around the clock, every dart off the target misses
		return "miss"
	case hit:
		return "hit"
	case last.Segment.IsBull():
		return "bull"
	case last.Segment == dartboard.Triple:
		return "triple"
	case last.Segment == dartboard.Double:
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
	ps, _ := st.Player(*p.CurrentPlayerID)
	return v.Game.Rules.(*x01.Settings).OutMode.Checkout(ps.Remaining, turn.DartsPerTurn-p.DartsInTurn)
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
	return dartCount(v) > len(v.Common().BullOff)
}

// dartCount counts the darts in the game's log, without takeouts and players leaving.
func dartCount(v service.GameView) int {
	n := 0
	for _, d := range v.Darts {
		if d.IsDart() {
			n++
		}
	}
	return n
}

// boardChoiceLabel is a board's label in the new game form; it says if a running game
// is on the board, which the new game then takes over.
func boardChoiceLabel(t Texts, b service.BoardView) string {
	if b.InUse {
		return "📷 " + b.Name + " (" + t.BoardInUse() + ")"
	}
	return "📷 " + b.Name
}

// joinURL is where the button to join an open game or tournament posts to.
func joinURL(l service.OpenLobby) string {
	if l.Tournament {
		return "/tournaments/" + strconv.FormatInt(l.ID, 10) + "/join"
	}
	return "/games/" + strconv.FormatInt(l.ID, 10) + "/join"
}

// boardVals are the form values of the button that puts a game on a board.
func boardVals(id int64) string {
	return `{"board_id": "` + strconv.FormatInt(id, 10) + `"}`
}

// myBoard returns the board the viewer's players throw on, ok is false if they enter
// their darts by hand.
func myBoard(v service.GameView) (id int64, name string, ok bool) {
	for _, pid := range v.Mine {
		if v.Common().Records[pid].Left {
			continue
		}
		if b := v.BoardOf(pid); b != nil {
			return *b, v.BoardNames[pid], true
		}
	}
	return 0, "", false
}

// sharedBoardName returns the name of the board if all players of the game throw on the
// same one (a game at one board), shown once at the top; empty otherwise.
func sharedBoardName(v service.GameView) string {
	var shared *int64
	for _, p := range v.Game.Participants {
		if p.BoardID == nil || (shared != nil && *shared != *p.BoardID) {
			return ""
		}
		shared = p.BoardID
	}
	if shared == nil {
		return ""
	}
	return v.BoardNames[v.Game.Participants[0].PlayerID]
}

// playerBoardName returns the name of the board a player throws on, if the players don't
// all throw on the same one (see sharedBoardName): then it is shown with each player.
func playerBoardName(v service.GameView, id int64) string {
	if sharedBoardName(v) != "" {
		return ""
	}
	return v.BoardNames[id]
}

// leaveLabel is the text of the button to leave a game: in a game of one account alone it
// ends the game, so it says "abort"; in a tournament it ends the tournament.
func leaveLabel(t Texts, v service.GameView) string {
	switch {
	case v.Game.TournamentID != nil:
		return t.LeaveTournament()
	case v.OnlyMine():
		return t.AbortGame()
	}
	return t.LeaveGame()
}

func leaveConfirm(t Texts, v service.GameView) string {
	switch {
	case v.Game.TournamentID != nil:
		return t.LeaveTournamentConfirm()
	case v.OnlyMine():
		return t.AbortGameConfirm()
	}
	return t.LeaveGameConfirm()
}

// mayEditDart reports whether the viewer may correct the dart: it is one of their
// players' (nobody corrects the darts of another account's player).
func mayEditDart(v service.GameView, d throw.Dart) bool {
	return v.CanThrow && v.MayEdit(slices.IndexFunc(v.Darts, func(e throw.Dart) bool { return e.ID == d.ID }))
}

// handEntry reports whether the viewer enters the darts of the player who is up by hand:
// that player is one of theirs and has no board with cameras. Then the page shows the
// keypad; otherwise the darts come from cameras or from another account, and the page
// gives the room to the board.
func handEntry(v service.GameView, p progress) bool {
	return p.CurrentPlayerID != nil && v.IsMine(*p.CurrentPlayerID) && !v.HasCamera(*p.CurrentPlayerID)
}

// mayAddDart reports whether the viewer may enter a dart that the cameras missed for the
// player who is up: it is one of theirs, and they have a board with cameras.
func mayAddDart(v service.GameView, p progress) bool {
	return v.CanThrow && p.CurrentPlayerID != nil && v.IsMine(*p.CurrentPlayerID) && v.HasCamera(*p.CurrentPlayerID)
}

// boardsOfViews and boardsOfChoices take the boards out of the service's views.
func boardsOfViews(views []service.BoardView) []board.Board {
	boards := make([]board.Board, len(views))
	for i, v := range views {
		boards[i] = v.Board
	}
	return boards
}

func boardsOfChoices(choices []service.BoardChoice) []board.Board {
	boards := make([]board.Board, len(choices))
	for i, c := range choices {
		boards[i] = c.Board
	}
	return boards
}

// defaultBoard is the ID of the board preselected for a new game: the one a game was
// last started on, else the first one (the list is sorted by ID); 0 without boards.
func defaultBoard(boards []board.Board) int64 {
	if len(boards) == 0 {
		return 0
	}
	last := boards[0]
	for _, b := range boards {
		if b.LastUsedAt != nil && (last.LastUsedAt == nil || b.LastUsedAt.After(*last.LastUsedAt)) {
			last = b
		}
	}
	return last.ID
}

// clock formats how long something took as players read it: 3:41, 1:02:05.
func clock(d time.Duration) string {
	s := int(d.Round(time.Second).Seconds())
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// setDuration returns how long a set took, if the leg is the set's last; 0 otherwise or
// while the set goes on. The history shows it with that leg.
func setDuration(st turn.State, set, leg int) time.Duration {
	if st.Times.LastLeg(set) != leg {
		return 0
	}
	return st.Times.Set(set).Duration()
}

// accent is how a player's accent color looks on the card of the player who is up.
// Tailwind only finds whole class names, so every color is written out.
type accent struct {
	Card   string // border, background and glow
	Badge  string // the "to throw" badge
	Swatch string // a color to choose in the settings
}

var accents = map[string]accent{
	"green":   {"border-board-green/70 from-board-green/25 [--glow-color:var(--color-board-green)]", "bg-board-green", "bg-board-green"},
	"red":     {"border-board-red/70 from-board-red/25 [--glow-color:var(--color-board-red)]", "bg-board-red", "bg-board-red"},
	"sky":     {"border-sky-400/70 from-sky-400/25 [--glow-color:var(--color-sky-400)]", "bg-sky-500", "bg-sky-400"},
	"violet":  {"border-violet-400/70 from-violet-400/25 [--glow-color:var(--color-violet-400)]", "bg-violet-500", "bg-violet-400"},
	"pink":    {"border-pink-400/70 from-pink-400/25 [--glow-color:var(--color-pink-400)]", "bg-pink-500", "bg-pink-400"},
	"orange":  {"border-orange-400/70 from-orange-400/25 [--glow-color:var(--color-orange-400)]", "bg-orange-500", "bg-orange-400"},
	"teal":    {"border-teal-400/70 from-teal-400/25 [--glow-color:var(--color-teal-400)]", "bg-teal-500", "bg-teal-400"},
	"indigo":  {"border-indigo-400/70 from-indigo-400/25 [--glow-color:var(--color-indigo-400)]", "bg-indigo-500", "bg-indigo-400"},
	"fuchsia": {"border-fuchsia-400/70 from-fuchsia-400/25 [--glow-color:var(--color-fuchsia-400)]", "bg-fuchsia-500", "bg-fuchsia-400"},
	"cyan":    {"border-cyan-400/70 from-cyan-400/25 [--glow-color:var(--color-cyan-400)]", "bg-cyan-500", "bg-cyan-400"},
}

// playerAccent returns the accent of a player of the game.
func playerAccent(v service.GameView, id int64) accent {
	for _, p := range v.Players {
		if p.ID == id {
			return accents[p.Accent()]
		}
	}
	return accents["green"]
}

// lastTurn is a player's last finished turn, shown in their tile.
type lastTurn struct {
	Darts  []throw.Dart
	Value  string // what it scored, in X01 the points or "Bust", in Around the Clock the hits
	Bust   bool
	Strong bool // 100 points or more
}

// lastTurnOf returns the last finished turn of a player, if they threw one.
func lastTurnOf(t Texts, v service.GameView, playerID int64) (lastTurn, bool) {
	switch st := v.State.(type) {
	case x01.State:
		for i := len(st.Turns) - 1; i >= 0; i-- {
			tr := st.Turns[i]
			if tr.PlayerID != playerID || !tr.Complete {
				continue
			}
			r := lastTurn{Darts: tr.Darts, Value: strconv.FormatUint(tr.Points, 10), Strong: tr.Points >= 100}
			if tr.Bust {
				r.Value, r.Bust = t.Busted(), true
			}
			return r, true
		}
	case aroundtheclock.State:
		for i := len(st.Turns) - 1; i >= 0; i-- {
			tr := st.Turns[i]
			if tr.PlayerID == playerID && tr.Complete {
				return lastTurn{Darts: tr.Darts, Value: strconv.Itoa(countTrue(tr.Hits))}, true
			}
		}
	}
	return lastTurn{}, false
}

// lobbyPlayers names the players that are in an open game or tournament already.
func lobbyPlayers(ctx context.Context, l service.OpenLobby) string {
	names := make([]string, len(l.Joined))
	for i, p := range l.Joined {
		names[i] = displayName(ctx, p)
	}
	return strings.Join(names, ", ")
}

// minutesLeft is how many more minutes an open game or tournament is offered, at least one.
func minutesLeft(l service.OpenLobby) int {
	left := service.OpenFor - time.Since(l.CreatedAt)
	return max(1, int((left+time.Minute-1)/time.Minute))
}

// filterValue is a number as the filter form has it: empty for none.
func filterValue(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// outModeValue is the check-out mode as the filter form has it: empty for none.
func outModeValue(m x01.OutMode) string {
	name, err := m.MarshalText()
	if err != nil {
		return ""
	}
	return string(name)
}

// filterOption is one choice of a select of the lobby filter.
type filterOption struct{ Value, Label string }

// numberOptions are the numbers from..to as choices.
func numberOptions(from, to int) []filterOption {
	var options []filterOption
	for n := from; n <= to; n++ {
		options = append(options, filterOption{strconv.Itoa(n), strconv.Itoa(n)})
	}
	return options
}
