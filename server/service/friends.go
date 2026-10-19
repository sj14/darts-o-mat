package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sj14/darts-o-mat/server/domain/account"
	"github.com/sj14/darts-o-mat/server/store"
)

var ErrAlreadyFriends = fmt.Errorf("%w: already friends or requested", ErrInvalid)

// Friend is another account as shown in the friend list.
type Friend struct {
	AccountID int64  `json:"account_id"`
	Username  string `json:"username"`
	Name      string `json:"name"` // name of the account's player
}

type FriendList struct {
	Friends  []Friend `json:"friends"`
	Incoming []Friend `json:"incoming"` // requests the account can accept
	Outgoing []Friend `json:"outgoing"` // requests the account sent and the other side hasn't answered yet
}

func (s *Service) Friends(ctx context.Context, acc account.Account) (FriendList, error) {
	friendships, err := s.store.ListFriendships(ctx, acc.ID)
	if err != nil {
		return FriendList{}, err
	}

	var ids []int64
	for _, f := range friendships {
		ids = append(ids, f.Other(acc.ID))
	}
	friends, err := s.friends(ctx, ids)
	if err != nil {
		return FriendList{}, err
	}

	list := FriendList{Friends: []Friend{}, Incoming: []Friend{}, Outgoing: []Friend{}}
	for i, f := range friendships {
		switch {
		case f.Status == account.FriendshipAccepted:
			list.Friends = append(list.Friends, friends[i])
		case f.AddresseeID == acc.ID:
			list.Incoming = append(list.Incoming, friends[i])
		default:
			list.Outgoing = append(list.Outgoing, friends[i])
		}
	}
	return list, nil
}

// RequestFriend sends a friend request to the account with the username.
// If that account already sent a request to acc, it is accepted instead.
func (s *Service) RequestFriend(ctx context.Context, acc account.Account, username string) error {
	username = account.NormalizeUsername(username)
	other, err := s.store.GetAccountByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		return UnknownUserError{Username: username}
	}
	if err != nil {
		return err
	}
	if other.ID == acc.ID {
		return fmt.Errorf("%w: you can't befriend yourself", ErrInvalid)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, err := s.store.GetFriendship(ctx, acc.ID, other.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return s.store.CreateFriendship(ctx, &account.Friendship{RequesterID: acc.ID, AddresseeID: other.ID, Status: account.FriendshipPending})
	case err != nil:
		return err
	case existing.Status == account.FriendshipPending && existing.AddresseeID == acc.ID:
		return s.store.AcceptFriendship(ctx, other.ID, acc.ID)
	default:
		return ErrAlreadyFriends
	}
}

// AcceptFriend accepts the friend request the other account sent, ErrNotFound if there is none.
func (s *Service) AcceptFriend(ctx context.Context, acc account.Account, otherID int64) error {
	return s.store.AcceptFriendship(ctx, otherID, acc.ID)
}

// RemoveFriend ends a friendship, or declines or withdraws a request.
// Games the two already play together are not affected.
func (s *Service) RemoveFriend(ctx context.Context, acc account.Account, otherID int64) error {
	return s.store.DeleteFriendship(ctx, acc.ID, otherID)
}

func (s *Service) areFriends(ctx context.Context, a, b int64) (bool, error) {
	f, err := s.store.GetFriendship(ctx, a, b)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return err == nil && f.Status == account.FriendshipAccepted, err
}

// friends returns the accounts as friends, in the order of ids.
func (s *Service) friends(ctx context.Context, ids []int64) ([]Friend, error) {
	accounts, err := s.store.GetAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	var playerIDs []int64
	for _, a := range accounts {
		playerIDs = append(playerIDs, a.PlayerID)
	}
	players, err := s.store.GetPlayers(ctx, playerIDs)
	if err != nil {
		return nil, err
	}

	names := map[int64]string{}
	for _, p := range players {
		names[p.ID] = p.Name
	}
	byID := map[int64]Friend{}
	for _, a := range accounts {
		byID[a.ID] = Friend{AccountID: a.ID, Username: a.Username, Name: names[a.PlayerID]}
	}

	result := []Friend{}
	for _, id := range ids {
		result = append(result, byID[id])
	}
	return result, nil
}
