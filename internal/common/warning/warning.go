// Package warning has the problems darts-o-cam reports about a board, which darts-o-mat
// shows on the game page. darts-o-cam always reports all of a board's current warnings,
// none if everything is fine.
package warning

import "encoding/json"

// Warning is one problem of a board; each kind is a type of its own.
type Warning interface {
	// Code names the kind in JSON.
	Code() string
}

// CamerasMissing: only Watching of the board's Cameras detect darts (none, e.g., when the
// detection stopped or is switched off).
type CamerasMissing struct {
	Cameras  int `json:"cameras"`
	Watching int `json:"watching"`
}

// SmallFrames: the frames of Camera are only Width×Height, much smaller than wanted,
// often because its USB connection is too slow.
type SmallFrames struct {
	Camera string `json:"camera"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// GridMismatch: the calibration of Camera doesn't fit the board anymore, e.g., the camera
// was moved.
type GridMismatch struct {
	Camera string `json:"camera"`
}

// Unknown is a warning of a kind this program doesn't know, from a newer darts-o-cam:
// the server shows a general warning for it.
type Unknown struct {
	Kind string
}

func (CamerasMissing) Code() string { return "cameras_missing" }
func (SmallFrames) Code() string    { return "small_frames" }
func (GridMismatch) Code() string   { return "grid_mismatch" }
func (u Unknown) Code() string      { return u.Kind }

// decoders decode the details of each known kind, by code.
var decoders = map[string]func(json.RawMessage) (Warning, error){
	CamerasMissing{}.Code(): decode[CamerasMissing],
	SmallFrames{}.Code():    decode[SmallFrames],
	GridMismatch{}.Code():   decode[GridMismatch],
}

func decode[T Warning](details json.RawMessage) (Warning, error) {
	var w T
	err := json.Unmarshal(details, &w)
	return w, err
}

// List is a board's warnings. In JSON each one is its code and its details:
// [{"code": "grid_mismatch", "details": {"camera": "cam2"}}].
type List []Warning

// encoded is a warning in JSON.
type encoded struct {
	Code    string          `json:"code"`
	Details json.RawMessage `json:"details,omitempty"`
}

func (l List) MarshalJSON() ([]byte, error) {
	out := make([]encoded, 0, len(l))
	for _, w := range l {
		e := encoded{Code: w.Code()}
		if _, unknown := w.(Unknown); !unknown {
			details, err := json.Marshal(w)
			if err != nil {
				return nil, err
			}
			e.Details = details
		}
		out = append(out, e)
	}
	return json.Marshal(out)
}

func (l *List) UnmarshalJSON(data []byte) error {
	var in []encoded
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	list := make(List, 0, len(in))
	for _, e := range in {
		decode, ok := decoders[e.Code]
		if !ok {
			list = append(list, Unknown{Kind: e.Code})
			continue
		}
		w, err := decode(e.Details)
		if err != nil {
			return err
		}
		list = append(list, w)
	}
	*l = list
	return nil
}
