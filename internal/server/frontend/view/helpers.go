package view

import (
	"context"
	"encoding"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/a-h/templ"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/common/warning"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/bot"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/cricket"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/service"
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

// playerName is the name of a player of the game as the game page shows it; a bot's with
// its spread in this game, "Bot (20.0 mm)".
func playerName(ctx context.Context, v service.GameView, id int64) string {
	for _, p := range v.Players {
		if p.ID != id {
			continue
		}
		if seat, _ := v.Game.Participant(id); seat.BotSpread > 0 {
			return displayName(ctx, p) + " (" + spreadMM(T(ctx), seat.BotSpread) + ")"
		}
		return displayName(ctx, p)
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
	case game.ModeCricket:
		label := t.Cricket()
		if r := g.Rules.(*cricket.Settings); r.Variant != cricket.VariantStandard {
			label += ", " + cricketVariantName(t, r.Variant)
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

// countWithShare returns the count and its share of the total, "120 (38%)", or only the
// count while the total is 0.
func countWithShare(ctx context.Context, count, total int) string {
	if total == 0 {
		return strconv.Itoa(count)
	}
	share := float64(count) / float64(total)
	return strconv.Itoa(count) + " (" + T(ctx).Percent(share) + ")"
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
	case cricket.State:
		for _, ps := range st.Players {
			lines = append(lines, playerLine{ps.PlayerID, ps.Sets, ps.SetLegs, strconv.Itoa(ps.Points), t.Points(), orDash(ps.Stats.Darts > 0, t.Decimal(ps.Stats.MPR)), t.MPR(), ps.Left})
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

// sum adds up the numbers, e.g. the marks of a turn.
func sum(ns []int) int {
	total := 0
	for _, n := range ns {
		total += n
	}
	return total
}

// dartClass marks a dart for coloring: "miss" for misses, "hit" for darts that hit the
// target in around the clock or scored a mark in cricket, empty otherwise.
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

// hitTarget reports whether a dart hit the player's target in around the clock, or scored a
// mark in cricket.
func hitTarget(v service.GameView, d throw.Dart) bool {
	hit, _ := targetResult(v, d)
	return hit
}

// targetState is the state of a mode in which every dart hits or misses: around the clock
// (the player's target) and cricket (a mark that counted).
type targetState interface {
	Hit(dartID int64) (hit, ok bool)
}

// targetResult reports for around the clock whether a dart hit the player's target, and
// for cricket whether it scored a mark; counted is false in other modes and for darts
// outside the turns (bull-off).
func targetResult(v service.GameView, d throw.Dart) (hit, counted bool) {
	if st, ok := v.State.(targetState); ok {
		return st.Hit(d.ID)
	}
	return false, false
}

// turnClock returns how many seconds of the time for the turn are gone and how many it
// has in total, if the turn is timed.
func turnClock(v service.GameView, p progress) (elapsed, total float64, ok bool) {
	if v.TurnDeadline == nil || !p.running() {
		return 0, 0, false
	}
	total = float64(v.Game.TurnSeconds)
	return min(max(total-time.Until(*v.TurnDeadline).Seconds(), 0), total), total, true
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
	case hit && v.Game.Mode == game.ModeAroundTheClock: // in cricket a scoring dart keeps its own sound
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

func presenceText(t Texts, p service.Presence) string {
	switch p {
	case service.PresencePlaying:
		return t.PresencePlaying()
	case service.PresenceOnline:
		return t.PresenceOnline()
	default:
		return t.PresenceOffline()
	}
}

// friendChoiceLabel is a friend's label in the lobby's invitation select; it says if
// the friend is online or playing.
func friendChoiceLabel(t Texts, f service.Friend) string {
	if f.Presence == service.PresenceOnline || f.Presence == service.PresencePlaying {
		return f.Name + " (" + presenceText(t, f.Presence) + ")"
	}
	return f.Name
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
	if p.CurrentPlayerID == nil || !v.IsMine(*p.CurrentPlayerID) {
		return false
	}
	if v.Game.IsBot(*p.CurrentPlayerID) {
		// A bot's turn keeps the layout: the keypad stays where nobody throws on a board,
		// for an account still in the game (its bot may play on after it was skipped out).
		return v.CanThrow && !v.HasBoard()
	}
	return !v.HasCamera(*p.CurrentPlayerID)
}

// mayEnterDart reports whether the viewer may enter the next dart: the game runs, they can
// throw and no takeout is pending.
func mayEnterDart(v service.GameView, p progress) bool {
	return p.running() && v.CanThrow && !p.WaitingForTakeout
}

// entersByHand reports whether the viewer enters the next dart by hand: they may
// (mayEnterDart) and it is the dart of one of their players without cameras (handEntry).
func entersByHand(v service.GameView, p progress) bool {
	return mayEnterDart(v, p) && handEntry(v, p)
}

// correcting reports whether the viewer corrects a dart: the page shows editPanel.
func correcting(v service.GameView, editing *throw.Dart) bool {
	return editing != nil && v.CanThrow
}

// boardAction is where a click on a field of the board posts its dart, "" if the board is
// only a picture. While a dart is corrected (correcting), the click corrects it like the
// keypad. Otherwise it enters the next dart if the viewer may (mayEnterDart) for one of
// their players, with or without cameras (for a dart the cameras missed), but not for a bot
// (botUp, the server throws for it) and not in the bull-off, where only the bull, the
// bullseye and a miss count (bullOffPanel).
func boardAction(v service.GameView, p progress, editing *throw.Dart) string {
	switch {
	case editing != nil:
		if correcting(v, editing) {
			return dartEditURL(v, *editing)
		}
		return ""
	case !p.BullingOff && !botUp(v, p) && mayEnterDart(v, p) && p.CurrentPlayerID != nil && v.IsMine(*p.CurrentPlayerID):
		return gameURL(v.Game.ID, "/darts")
	}
	return ""
}

// fieldVals are the form values of a click on a field of the board.
func fieldVals(f boardField) string {
	return `{"number":"` + strconv.Itoa(int(f.Number)) + `","segment":"` + f.Segment.String() + `"}`
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
	"green":  {"border-board-green/70 from-board-green/25 [--glow-color:var(--color-board-green)]", "bg-board-green", "bg-board-green"},
	"red":    {"border-board-red/70 from-board-red/25 [--glow-color:var(--color-board-red)]", "bg-board-red", "bg-board-red"},
	"sky":    {"border-sky-400/70 from-sky-400/25 [--glow-color:var(--color-sky-400)]", "bg-sky-500", "bg-sky-400"},
	"violet": {"border-violet-400/70 from-violet-400/25 [--glow-color:var(--color-violet-400)]", "bg-violet-500", "bg-violet-400"},
	"pink":   {"border-pink-400/70 from-pink-400/25 [--glow-color:var(--color-pink-400)]", "bg-pink-500", "bg-pink-400"},
	"orange": {"border-orange-400/70 from-orange-400/25 [--glow-color:var(--color-orange-400)]", "bg-orange-500", "bg-orange-400"},
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
	Max    bool // 180
}

// lastTurnOf returns the last finished turn of a player, if they threw one.
func lastTurnOf(t Texts, v service.GameView, playerID int64) (lastTurn, bool) {
	switch st := v.State.(type) {
	case x01.State:
		for _, tr := range slices.Backward(st.Turns) {
			if tr.PlayerID != playerID || !tr.Complete {
				continue
			}
			r := lastTurn{Darts: tr.Darts, Value: strconv.FormatUint(tr.Points, 10), Strong: tr.Points >= 100, Max: tr.Points == 180}
			if tr.Bust {
				r.Value, r.Bust = t.Busted(), true
			}
			return r, true
		}
	case aroundtheclock.State:
		for _, tr := range slices.Backward(st.Turns) {
			if tr.PlayerID != playerID || !tr.Complete {
				continue
			}
			return lastTurn{Darts: tr.Darts, Value: strconv.Itoa(countTrue(tr.Hits))}, true
		}
	case cricket.State:
		for _, tr := range slices.Backward(st.Turns) {
			if tr.PlayerID != playerID || !tr.Complete {
				continue
			}
			r := lastTurn{Darts: tr.Darts, Value: t.NMarks(sum(tr.Marks))}
			// In Cut-Throat the points went to the opponents: not the player's score.
			if tr.Points > 0 && st.Variant == cricket.VariantStandard {
				r.Value += " · " + strconv.Itoa(tr.Points)
			}
			return r, true
		}
	}
	return lastTurn{}, false
}

// visitScore is the score of the visit the log's last entry finished (the third dart, a
// bust, a checkout), shown large over the board (visitScoreView), a 180 as "180!". Not for
// a won leg, set or game (their banners), nor in the bull-off, whose darts are in no visit.
func visitScore(t Texts, v service.GameView, p progress) (lastTurn, bool) {
	// A won set has its leg winner as well.
	if len(v.Darts) == 0 || !p.running() || p.LegWinner != nil {
		return lastTurn{}, false
	}
	last := v.Darts[len(v.Darts)-1]
	visit, ok := lastTurnOf(t, v, last.PlayerID)
	if !ok || !isLatest(v, visit.Darts[len(visit.Darts)-1]) {
		return lastTurn{}, false
	}
	if visit.Max {
		visit.Value += "!"
	}
	return visit, true
}

// visitScoreClasses animate and color the visit's score: a 180 celebrates in gold, the
// others fade in and out, red for a bust, gold for 100 points or more, else cream.
func visitScoreClasses(score lastTurn) string {
	switch {
	case score.Max:
		return "animate-celebrate text-amber-300"
	case score.Bust:
		return "animate-visit-score text-board-red"
	case score.Strong:
		return "animate-visit-score text-amber-300"
	}
	return "animate-visit-score text-board-cream"
}

// lobbyPlayers names the players that are in an open game or tournament already, each with
// their form in its mode ("new" without enough games) and their thumbs.
func lobbyPlayers(ctx context.Context, l service.OpenLobby) string {
	t := T(ctx)
	names := make([]string, len(l.Joined))
	for i, p := range l.Joined {
		form := t.NewPlayer()
		if f, ok := l.Forms[p.ID]; ok {
			switch l.Settings.Mode {
			case game.ModeAroundTheClock:
				form = t.FormHitRate(t.Percent(f))
			case game.ModeCricket:
				form = t.FormMPR(t.Decimal(f))
			default:
				form = t.FormAverage(t.Decimal(f))
			}
		}
		names[i] = displayName(ctx, p) + " (" + form + ")"
		if r, ok := l.Reputations[p.ID]; ok {
			names[i] += " 👍 " + T(ctx).Percent(r.Share()) + " (" + strconv.Itoa(r.Total()) + ")"
		}
	}
	return strings.Join(names, ", ")
}

// thumbValue is the form value of a thumb button.
func thumbValue(up bool) string {
	if up {
		return `{"up":"1"}`
	}
	return `{"up":"0"}`
}

// waiting is how long an open game or tournament has been waiting for players: minutes
// (at least one) for the first hour, hours for the first two days, then days.
func waiting(t Texts, l service.OpenLobby) string {
	d := time.Since(l.CreatedAt)
	switch {
	case d < time.Hour:
		return t.WaitingMinutes(max(1, int(d/time.Minute)))
	case d < 48*time.Hour:
		return t.WaitingHours(int(d / time.Hour))
	}
	return t.WaitingDays(int(d / (24 * time.Hour)))
}

// filterValue is a number as the filter form has it: empty for none.
func filterValue(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// formValue is the strength filter as the filter form has it: yes, or empty for any.
func formValue(fits bool) string {
	if fits {
		return string(service.Yes)
	}
	return ""
}

// textValue is a setting (check-out mode, cricket variant) as the forms have it: empty for
// none.
func textValue(m encoding.TextMarshaler) string {
	name, err := m.MarshalText()
	if err != nil {
		return ""
	}
	return string(name)
}

// cricketVariantName names who scores in a variant of cricket.
func cricketVariantName(t Texts, v cricket.Variant) string {
	if v == cricket.VariantCutThroat {
		return t.CricketCutThroat()
	}
	return t.CricketStandard()
}

// filterOption is one choice of a select of the lobby filter.
type filterOption struct{ Value, Label string }

// yesNoOptions are the choices of a service.Choice besides "any".
func yesNoOptions(t Texts) []filterOption {
	return []filterOption{{string(service.Yes), t.FilterYes()}, {string(service.No), t.FilterNo()}}
}

// numberOptions are the numbers from..to as choices.
func numberOptions(from, to int) []filterOption {
	var options []filterOption
	for n := from; n <= to; n++ {
		options = append(options, filterOption{strconv.Itoa(n), strconv.Itoa(n)})
	}
	return options
}

// showMode tells whether the statistics of a mode are shown when the page is narrowed down
// to selected, which is empty for all modes.
func showMode(selected, m game.Mode) bool {
	return selected == "" || selected == m
}

// botStrength is about what a bot with the spread plays in the mode, the way the
// statistics count it: the average in X01, the hit rate in around the clock, the MPR in
// cricket (bot.Strength).
func botStrength(t Texts, mode game.Mode, spread game.Spread) string {
	strength := bot.Strength(mode, spread)
	switch mode {
	case game.ModeAroundTheClock:
		return t.BotHitRate(strength)
	case game.ModeCricket:
		return t.BotMPR(strength)
	}
	return t.BotAverage(strength)
}

// botUp reports whether a bot is up: the server throws for it, nobody enters its darts.
func botUp(v service.GameView, p progress) bool {
	return p.CurrentPlayerID != nil && v.Game.IsBot(*p.CurrentPlayerID)
}

// spreadMM is a bot's spread in mm, to a tenth, in the language: "12.5 mm".
func spreadMM(t Texts, spread game.Spread) string {
	return t.Decimal(spread.MM()) + " mm"
}

// botSliderValue is what the slider that chooses a bot's strength shows for its position:
// the spread and about what it plays in the mode, "12.5 mm ≈ 63 average".
func botSliderValue(t Texts, mode game.Mode, position int) string {
	spread := bot.SpreadAt(position)
	return spreadMM(t, spread) + " " + botStrength(t, mode, spread)
}

// statsURL is the statistics page narrowed down to a mode (empty for all) and opponents.
func statsURL(mode game.Mode, opponents stats.Opponents) string {
	url := "/stats?mode=" + string(mode)
	if opponents != stats.AgainstPeople {
		url += "&opponents=" + string(opponents)
	}
	return url
}

// turnLabel names a step of x01.TurnScores: 60+, …, 180 (which can't be exceeded).
func turnLabel(i int) string {
	label := strconv.FormatUint(x01.TurnScores[i], 10)
	if i < len(x01.TurnScores)-1 {
		label += "+"
	}
	return label
}

// turnCounts lists the turns per step below 180, e.g. "12 / 5 / 1 / 0" for 60+ to 170+;
// the 180s have a tile of their own.
func turnCounts(t x01.Turns) (label, value string) {
	var labels, values []string
	for i, n := range t[:len(t)-1] {
		labels, values = append(labels, turnLabel(i)), append(values, strconv.Itoa(n))
	}
	return strings.Join(labels, " / "), strings.Join(values, " / ")
}

// rulesOf returns the rules of the settings if they are R's, else R's zero value.
func rulesOf[R any](s game.Settings) R {
	if r, ok := any(s.Rules).(*R); ok {
		return *r
	}
	var zero R
	return zero
}

// warningText is what the game page says about a warning of a board; a code this server
// doesn't know (from a newer darts-o-cam) gets a general text.
func warningText(t Texts, boardName string, w warning.Warning) string {
	switch w := w.(type) {
	case warning.CamerasMissing:
		return t.CamerasMissing(boardName, w.Watching, w.Cameras)
	case warning.SmallFrames:
		return t.SmallFrames(boardName, w.Camera, w.Width, w.Height)
	case warning.GridMismatch:
		return t.GridMismatch(boardName, w.Camera)
	default: // warning.Unknown
		return t.BoardWarning(boardName)
	}
}
