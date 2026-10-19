package x01

import (
	"strconv"
	"strings"

	"github.com/sj14/darts-o-mat/common/throw"
)

// doubleOutTable holds the usual three-dart routes for double out from 170 to 41,
// from the checkout table of Guido Wessendorf
// (https://www.uni-muenster.de/IT.GuidoWessendorf/checkout.html, first route where it
// lists several). Scores without an entry can't be finished with three darts.
var doubleOutTable = map[uint64]string{
	170: "T20 T20 DB", 167: "T20 T19 DB", 164: "T20 T18 DB", 161: "T20 T17 DB",
	160: "T20 T20 D20", 158: "T20 T20 D19", 157: "T20 T19 D20", 156: "T20 T20 D18",
	155: "T20 T19 D19", 154: "T20 T18 D20", 153: "T20 T19 D18", 152: "T20 T20 D16",
	151: "T20 T17 D20", 150: "T20 T18 D18", 149: "T20 T19 D16", 148: "T20 T20 D14",
	147: "T20 T17 D18", 146: "T20 T18 D16", 145: "T20 T19 D14", 144: "T20 T20 D12",
	143: "T20 T17 D16", 142: "T20 T14 D20", 141: "T20 T19 D12", 140: "T20 T20 D10",
	139: "T20 T13 D20", 138: "T20 T18 D12", 137: "T20 T19 D10", 136: "T20 T20 D8",
	135: "T20 T17 D12", 134: "T20 T16 D13", 133: "T20 T19 D8", 132: "T20 T16 D12",
	131: "T19 T14 D16", 130: "T20 T20 D5", 129: "T19 T16 D12", 128: "T18 T14 D16",
	127: "T20 T17 D8", 126: "T19 T19 D6", 125: "T18 T19 D7", 124: "T20 T14 D11",
	123: "T19 T16 D9", 122: "T18 T18 D7", 121: "T20 T11 D14", 120: "T20 20 D20",
	119: "T19 T12 D13", 118: "T20 18 D20", 117: "T19 20 D20", 116: "T19 19 D20",
	115: "T20 15 D20", 114: "T19 17 D20", 113: "T19 16 D20", 112: "T20 T12 D8",
	111: "T19 14 D20", 110: "T20 T10 D10", 109: "T20 9 D20", 108: "T20 16 D16",
	107: "T19 T10 D10", 106: "T20 T10 D8", 105: "T20 13 D16", 104: "T19 15 D16",
	103: "T19 6 D20", 102: "T20 10 D16", 101: "T20 9 D16", 100: "T20 D20",
	99: "T19 10 D16", 98: "T20 D19", 97: "T19 D20", 96: "T20 D18",
	95: "T19 D19", 94: "T18 D20", 93: "T19 D18", 92: "T20 D16",
	91: "T17 D20", 90: "T20 D15", 89: "T19 D16", 88: "T20 D14",
	87: "T17 D18", 86: "T18 D16", 85: "T15 D20", 84: "T20 D12",
	83: "T17 D16", 82: "DB D16", 81: "T19 D12", 80: "T20 D10",
	79: "T19 D11", 78: "T18 D12", 77: "T19 D10", 76: "T20 D8",
	75: "T17 D12", 74: "T14 D16", 73: "T19 D8", 72: "T16 D12",
	71: "T13 D16", 70: "T18 D8", 69: "T19 D6", 68: "T20 D4",
	67: "T9 D20", 66: "T10 D18", 65: "T11 D16", 64: "T16 D8",
	63: "T13 D12", 62: "T10 D16", 61: "T15 D8", 60: "20 D20",
	59: "19 D20", 58: "18 D20", 57: "17 D20", 56: "T16 D4",
	55: "15 D20", 54: "14 D20", 53: "13 D20", 52: "12 D20",
	51: "11 D20", 50: "10 D20", 49: "9 D20", 48: "16 D16",
	47: "7 D20", 46: "6 D20", 45: "13 D16", 44: "12 D16",
	43: "3 D20", 42: "10 D16", 41: "9 D16",
}

// doubleOutRoute returns the usual route for double out: from the table above 40, below
// a double directly, or for odd scores a single that leaves 32, 16, 8, 4 or 2.
func doubleOutRoute(remaining uint64) []throw.Dart {
	if route, ok := doubleOutTable[remaining]; ok {
		return parseRoute(route)
	}
	if remaining < 2 || remaining > 40 {
		return nil
	}
	if remaining%2 == 0 {
		return []throw.Dart{{Number: uint8(remaining / 2), Segment: throw.Double}}
	}
	leave := uint64(32)
	for leave >= remaining {
		leave /= 2
	}
	return []throw.Dart{{Number: uint8(remaining - leave), Segment: throw.Single}, {Number: uint8(leave / 2), Segment: throw.Double}}
}

// parseRoute reads a route like "T20 10 D16"; DB is the bullseye.
func parseRoute(route string) []throw.Dart {
	var darts []throw.Dart
	for f := range strings.FieldsSeq(route) {
		d := throw.Dart{Segment: throw.Single}
		switch f[0] {
		case 'T':
			d.Segment, f = throw.Triple, f[1:]
		case 'D':
			d.Segment, f = throw.Double, f[1:]
		}
		if f == "B" {
			d.Number = throw.BullNumber
			d.Segment = map[throw.Segment]throw.Segment{throw.Single: throw.Bull, throw.Double: throw.Bullseye}[d.Segment]
		} else {
			n, _ := strconv.Atoi(f)
			d.Number = uint8(n)
		}
		darts = append(darts, d)
	}
	return darts
}
