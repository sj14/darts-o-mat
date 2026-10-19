package dartboard

import (
	"encoding/json"
	"testing"
)

func TestHitString(t *testing.T) {
	tests := []struct {
		dart Hit
		want string
	}{
		{Hit{Number: 20, Segment: Triple}, "T20"},
		{Hit{Number: 20, Segment: Inner}, "20"},
		{Hit{Number: BullNumber, Segment: Bullseye}, "Bullseye"},
		{Hit{Segment: Miss, Number: 7}, "M7"},
		{Hit{Segment: Miss}, "Miss"},
	}
	for _, tt := range tests {
		if got := tt.dart.String(); got != tt.want {
			t.Errorf("%+v: got %q, want %q", tt.dart, got, tt.want)
		}
	}
}

func TestHitScore(t *testing.T) {
	tests := []struct {
		dart Hit
		want uint64
	}{
		{Hit{Number: 20, Segment: Triple}, 60},
		{Hit{Number: 20, Segment: Outer}, 20},
		{Hit{Number: 5, Segment: Double}, 10},
		{Hit{Number: BullNumber, Segment: Bull}, 25},
		{Hit{Number: BullNumber, Segment: Bullseye}, 50},
		{Hit{Number: 7, Segment: Miss}, 0},
	}
	for _, tt := range tests {
		if got := tt.dart.Score(); got != tt.want {
			t.Errorf("%v: %d points, want %d", tt.dart, got, tt.want)
		}
	}
}

func TestHitValidate(t *testing.T) {
	valid := []Hit{
		{Segment: Miss, Number: 20},
		{Segment: Miss},
		{Segment: Single, Number: 1},
		{Segment: Bullseye, Number: BullNumber},
	}
	for _, d := range valid {
		if err := d.Validate(); err != nil {
			t.Errorf("%+v: %v", d, err)
		}
	}
	invalid := []Hit{
		{Segment: Miss, Number: BullNumber},   // beside the bull
		{Segment: Triple, Number: BullNumber}, // no triple bull
		{Segment: Bull, Number: 20},           // the bull is 25
		{Segment: Double, Number: 21},
		{Number: 20},                       // no segment
		{Segment: Bullseye + 1, Number: 1}, // unknown segment
	}
	for _, d := range invalid {
		if err := d.Validate(); err == nil {
			t.Errorf("%+v: no error", d)
		}
	}
}

func TestSegmentJSON(t *testing.T) {
	data, err := json.Marshal(Hit{Number: 20, Segment: Outer})
	if err != nil {
		t.Fatal(err)
	}
	var d Hit
	if err := json.Unmarshal(data, &d); err != nil || d.Segment != Outer {
		t.Errorf("%s: got %+v, %v", data, d, err)
	}
	if err := json.Unmarshal([]byte(`{"number":20,"segment":"quadruple"}`), &d); err == nil {
		t.Error("unknown segment: no error")
	}
}
