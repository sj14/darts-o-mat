package throw

import (
	"encoding/json"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

func TestJSON(t *testing.T) {
	// A takeout has no segment, and none in its JSON.
	if data, _ := json.Marshal(Dart{Takeout: true}); string(data) != `{"id":0,"number":0,"takeout":true}` {
		t.Errorf("takeout: %s", data)
	}
}

// The log entry scores like the hit it holds.
func TestHit(t *testing.T) {
	d := FromHit(dartboard.Hit{Number: 20, Segment: dartboard.Triple})
	if d.Score() != 60 || d.String() != "T20" || d.Validate() != nil || d.Hit() != (dartboard.Hit{Number: 20, Segment: dartboard.Triple}) {
		t.Errorf("%+v: %d points, %q, %v", d, d.Score(), d.String(), d.Validate())
	}
}
