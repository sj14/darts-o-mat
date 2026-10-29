package warning

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestListJSON(t *testing.T) {
	t.Parallel()
	list := List{CamerasMissing{Cameras: 3, Watching: 2}, SmallFrames{Camera: "cam1", Width: 160, Height: 120}, GridMismatch{Camera: "cam2"}}
	data, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"code":"cameras_missing","details":{"cameras":3,"watching":2}},` +
		`{"code":"small_frames","details":{"camera":"cam1","width":160,"height":120}},` +
		`{"code":"grid_mismatch","details":{"camera":"cam2"}}]`
	if string(data) != want {
		t.Errorf("JSON %s, want %s", data, want)
	}
	var back List
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(back, list) {
		t.Errorf("decoded %#v, want %#v", back, list)
	}
}

// A kind of a newer darts-o-cam is kept as Unknown, without its details.
func TestListJSONUnknown(t *testing.T) {
	t.Parallel()
	var list List
	if err := json.Unmarshal([]byte(`[{"code":"too_dark","details":{"camera":"cam1"}}]`), &list); err != nil {
		t.Fatal(err)
	}
	if want := (List{Unknown{Kind: "too_dark"}}); !slices.Equal(list, want) {
		t.Errorf("decoded %#v, want %#v", list, want)
	}
	if data, _ := json.Marshal(list); string(data) != `[{"code":"too_dark"}]` {
		t.Errorf("JSON %s, want only the code", data)
	}
	if err := json.Unmarshal([]byte(`[{"code":"grid_mismatch","details":[]}]`), &list); err == nil {
		t.Error("details of the wrong shape: no error")
	}
}
