package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/bot"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
)

// A bot is a guest of the account that adds it (player.Player.Bot), whom the server throws
// for (botturn.go). Bots play only in games the players set up themselves, never among
// themselves: an account's bots leave with its last player who isn't a bot.

var (
	// ErrNoBots means a bot can't take the seat: bots only play in games that aren't open
	// for strangers, and not in tournaments.
	ErrNoBots = fmt.Errorf("%w: bots only play in games that aren't open for strangers", ErrInvalid)
	// ErrBotBoard means a bot was to be put on a board: it throws on none.
	ErrBotBoard = fmt.Errorf("%w: a bot throws on no board", ErrInvalid)
	// ErrBotSpread means the spread of a bot is outside bot.MinSpread to bot.MaxSpread.
	ErrBotSpread = fmt.Errorf("%w: a bot's spread must be between %v and %v mm", ErrInvalid, bot.MinSpread.MM(), bot.MaxSpread.MM())
	// ErrTooManyBots means all MaxBots bots of the account are seated: no other one is made.
	ErrTooManyBots = fmt.Errorf("%w: at most %d bots", ErrInvalid, MaxBots)
)

// MaxBots is how many bots an account can have, like MaxGuests for its guests: games that
// aren't open have no limit of seats, so without it an account could make any number.
const MaxBots = 5

// AddBot seats a bot of the account in the lobby of a game it plays in, with the spread
// of its darts (bot.Throw). It takes a bot of the account that has no seat right
// now and makes another one only if all are seated, so an account keeps only as many bots
// as it needed at once.
func (s *Service) AddBot(ctx context.Context, acc account.Account, gameID int64, spread game.Spread) error {
	unlock, err := s.lock(ctx, "AddBot")
	if err != nil {
		return err
	}
	defer unlock()
	l, pt, err := s.lobbyFor(ctx, acc, GameLobby(gameID))
	if err != nil {
		return err
	}
	if l.humans(pt.mine) == 0 {
		return ErrForbidden // a bot plays only next to a player of its account
	}
	if l.Open() {
		return ErrNoBots
	}
	if bot.CheckSpread(spread) != nil {
		return ErrBotSpread // before freeBot, which may make a bot
	}
	botID, err := s.freeBot(ctx, acc, playerIDs(l.seats))
	if err != nil {
		return err
	}
	seat := game.Participant{PlayerID: botID, Status: game.InvitationAccepted, BotSpread: spread}
	return s.takeSeats(ctx, acc, l.seat(), []game.Participant{seat}, 0, func() error {
		return s.addSeat(ctx, l, seat)
	})
}

// freeBot returns a bot of the account that has no seat and isn't one of taken (the seats
// of the lobby), else it makes another one, up to MaxBots (ErrTooManyBots). A new bot that
// then can't take its seat is free for the next time. The caller holds the lock (s.lock).
func (s *Service) freeBot(ctx context.Context, acc account.Account, taken []int64) (int64, error) {
	bots, err := s.store.ListBots(ctx, acc.ID)
	if err != nil {
		return 0, err
	}
	for _, b := range bots {
		if slices.Contains(taken, b.ID) {
			continue
		}
		err := s.checkPlayersFree(ctx, acc, []int64{b.ID}, 0)
		if err == nil {
			return b.ID, nil
		}
		if !errors.As(err, &PlayerBusyError{}) {
			return 0, err
		}
	}
	if len(bots) >= MaxBots {
		return 0, ErrTooManyBots
	}
	name := "Bot"
	if len(bots) > 0 {
		name += " " + strconv.Itoa(len(bots)+1)
	}
	b := player.Player{Name: name, GuestOf: &acc.ID, Bot: true}
	err = s.store.StorePlayer(ctx, &b)
	return b.ID, err
}

// checkBotSeats checks where bots sit: only in a game that isn't open for strangers and
// isn't part of a tournament (ErrNoBots), on no board (ErrBotBoard), with a spread
// (ErrBotSpread); and that only a bot has a spread.
func (s *Service) checkBotSeats(ctx context.Context, st seat, seats []game.Participant) error {
	players, err := s.store.GetPlayers(ctx, playerIDs(seats))
	if err != nil {
		return err
	}
	for _, p := range seats {
		isBot := slices.ContainsFunc(players, func(pl player.Player) bool { return pl.ID == p.PlayerID && pl.Bot })
		switch {
		case !isBot && p.BotSpread != 0:
			return fmt.Errorf("%w: only a bot has a spread", ErrInvalid)
		case !isBot:
		case st.tournament || st.lobby.Open():
			return ErrNoBots
		case p.BoardID != nil:
			return ErrBotBoard
		case bot.CheckSpread(p.BotSpread) != nil:
			return ErrBotSpread
		}
	}
	return nil
}

// onlyBotsLeft reports whether every player still in the game, in the state after some
// players left, is a bot: then the game is aborted, as bots don't play among themselves.
func onlyBotsLeft(g game.Game, st game.State) bool {
	for _, p := range g.Participants {
		if p.BotSpread == 0 && !st.Common().Records[p.PlayerID].Left {
			return false
		}
	}
	return true
}
