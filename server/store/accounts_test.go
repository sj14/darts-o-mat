package store

import (
	"context"
	"testing"

	"github.com/sj14/darts-o-mat/server/domain/account"
)

// The unique index keeps an email to one account even when the check before it is skipped,
// as when two requests race, and lets any number of accounts have none.
func TestEmailIndex(t *testing.T) {
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	insert := func(username, email string) error {
		a := account.Account{Username: username, Email: email, PasswordHash: "x", PlayerID: int64(len(username))}
		_, err := st.db.NewInsert().Model(&a).Exec(ctx)
		return err
	}
	if err := insert("aaa", ""); err != nil {
		t.Fatal(err)
	}
	if err := insert("bbbb", ""); err != nil {
		t.Errorf("a second account without an email: %v", err)
	}
	if err := insert("ccccc", "c@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := insert("dddddd", "c@example.com"); err == nil {
		t.Error("two accounts with the same email were stored")
	}
}
