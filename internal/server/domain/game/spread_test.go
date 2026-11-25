package game

import (
	"encoding/json"
	"testing"
)

// A spread is the number of mm in JSON, rounded to a tenth.
func TestSpreadJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		json string
		want Spread
	}{
		{"12.5", 125},
		{"20", 200},
		{"0", 0},
		{"4.04", 40},
		{"4.06", 41},
	}
	for _, tt := range tests {
		var s Spread
		if err := json.Unmarshal([]byte(tt.json), &s); err != nil || s != tt.want {
			t.Errorf("%s: %v, %v, want %d", tt.json, s, err, tt.want)
		}
	}
	for _, s := range []Spread{125, 200, 40, 800} {
		data, err := json.Marshal(s)
		var back Spread
		if err != nil || json.Unmarshal(data, &back) != nil || back != s {
			t.Errorf("%d: %s comes back as %d (%v)", s, data, back, err)
		}
	}
	if data, _ := json.Marshal(Spread(125)); string(data) != "12.5" {
		t.Errorf("125 is %s in JSON, want 12.5", data)
	}
	kept := Spread(125)
	if err := kept.UnmarshalJSON([]byte("null")); err != nil || kept != 125 {
		t.Errorf("null: %d, %v, want 125 kept without error", kept, err)
	}
	var s Spread
	for _, bad := range []string{`"x"`, `null1`, "1e99"} {
		if err := json.Unmarshal([]byte(bad), &s); err == nil {
			t.Errorf("%s was read as %d, want an error", bad, s)
		}
	}
}
