package camapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/common/warning"
)

// errUnreachable means a report certainly didn't reach the server: the connection was
// refused (the server restarts) or couldn't be made (the network is gone), or a proxy
// answered that it has no server behind it (502, 503). It is safe to send it again, the
// server can't have entered it. A timeout or a connection lost on the way isn't: the
// server may have entered the dart, and sending it again would count it twice.
var errUnreachable = errors.New("server not reachable")

// server reports darts to darts-o-mat with the board's token.
type server struct {
	url    string
	token  string
	client *http.Client
}

func newServer(url, token string) *server {
	return &server{url: strings.TrimSuffix(url, "/"), token: token, client: &http.Client{Timeout: 10 * time.Second}}
}

// dart enters a dart into the running game on the board and returns the game's id.
func (s *server) dart(ctx context.Context, d dartboard.Hit) (int64, error) {
	body, err := json.Marshal(d)
	if err != nil {
		return 0, err
	}
	return s.post(ctx, "/api/board/darts", body)
}

// takeout reports that the darts were pulled out.
func (s *server) takeout(ctx context.Context) (int64, error) {
	return s.post(ctx, "/api/board/takeout", nil)
}

// warnings reports all current warnings of the board, none if everything is fine.
func (s *server) warnings(ctx context.Context, ws warning.List) error {
	body, err := json.Marshal(struct {
		Warnings warning.List `json:"warnings"`
	}{ws})
	if err != nil {
		return err
	}
	_, err = s.send(ctx, http.MethodPut, "/api/board/warnings", body)
	return err
}

// post sends a dart or a takeout and returns the id of the game it was entered in.
func (s *server) post(ctx context.Context, path string, body []byte) (int64, error) {
	data, err := s.send(ctx, http.MethodPost, path, body)
	if err != nil {
		return 0, err
	}
	var v struct {
		Game struct {
			ID int64 `json:"id"`
		} `json:"game"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, err
	}
	return v.Game.ID, nil
}

// send sends a request with the board's token and returns the body of the answer.
func (s *server) send(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.url+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if notConnected(err) {
			return nil, fmt.Errorf("%w: %w", errUnreachable, err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable: // not 504: the server may have it
		return nil, fmt.Errorf("%w: %s", errUnreachable, resp.Status)
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return nil, fmt.Errorf("server: %s", e.Error)
		}
		return nil, fmt.Errorf("server: %s", resp.Status)
	}
	return data, nil
}

// notConnected reports whether a request failed before it was sent: no connection could
// be made (refused, no route, unknown host).
func notConnected(err error) bool {
	op, ok := errors.AsType[*net.OpError](err)
	return ok && op.Op == "dial"
}
