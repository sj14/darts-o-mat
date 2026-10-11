package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

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
func (s *server) dart(d throw.Dart) (int64, error) {
	body, err := json.Marshal(d)
	if err != nil {
		return 0, err
	}
	return s.post("/api/board/darts", body)
}

// takeout reports that the darts were pulled out.
func (s *server) takeout() (int64, error) {
	return s.post("/api/board/takeout", nil)
}

func (s *server) post(path string, body []byte) (int64, error) {
	req, err := http.NewRequest(http.MethodPost, s.url+path, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return 0, fmt.Errorf("server: %s", e.Error)
		}
		return 0, fmt.Errorf("server: %s", resp.Status)
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
