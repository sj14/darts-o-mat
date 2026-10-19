package camui

import (
	"context"
	"fmt"
	"image/color"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/i18n"
	"github.com/sj14/darts-o-mat/pkg/vision"
)

// Texts are the texts of darts-o-cam's web interface, one method per text, like the
// server's (see pkg/i18n): English (textsEN) implements all of them, the other
// languages embed it and override what is translated. The language is the browser's.
type Texts interface {
	Number(f float64, digits int) string

	// Frame and status.
	NavCameras() string
	NavAddCamera() string
	NavSettings() string
	NewVersion(version string) string
	Back() string
	DetectionRunning(cameras int) string
	DetectionWaiting() string
	DetectionStopped(err string) string
	DetectionOff() string

	// Overview.
	TurnDetectionOff() string
	TurnDetectionOn() string
	ThrowsGoTo() string
	NotConnectedBefore() string
	NotConnectedAfter() string
	NoCameras() string
	CalibrateHintBefore() string
	EmptyBoard() string
	CalibrateHintAfter() string
	RefreshImages() string
	Reconnect() string
	ReconnectHint() string
	ImageOf(name string) string
	PixelsPerMM() string
	Calibrated() string
	NotCalibrated() string
	CalibratedForOtherSize(size string) string
	Recalibrate() string
	Calibrate() string
	CheckGrid() string
	RefineOnly() string
	Remove() string
	LatestDetections() string
	ConfigHint(dir string) string
	ProblemNotConnected() string
	ProblemMoved() string

	// Detections.
	NoEvents() string
	DartsPulledOut() string
	NCameras(n int) string
	NotReported(err string) string
	ReportedInGame(game string) string
	Miss() string
	DetectionTitle() string
	UnlocatedNotice() string
	Camera() string
	PositionMM() string
	DistanceToCenter() string
	Resolution() string
	Field() string
	Hidden() string
	Result() string
	EventExplanation() string
	Before() string
	After() string

	// Detection in detail (trace).
	TraceTitle() string
	TraceHint() string
	LegendSeed() string
	LegendArea() string
	LegendPiece() string
	LegendChanged() string
	LegendEarlier() string
	LegendFirstAxis() string
	LegendAxis() string
	LegendTipEnd() string
	LegendTip() string
	TraceChanged(pixels, pieces int) string
	TraceDart(seed, area int, elongation string) string
	TraceRetried() string
	TraceNoDart(err string) string
	Zoom() string
	WholeImage() string

	// Removing a camera.
	RemoveTitle(name string) string
	RemoveQuestion(name string) string
	RemoveHint() string
	Cancel() string

	// Calibration.
	CalibrateTitle(name string) string
	CalibrateHeading(name string) string
	ClickBefore() string
	And() string
	ClickAfter() string
	ClickHint() string
	AllPointsDone() string
	PointsSet(points, min int) string
	Skip() string
	FinishCalibration() string
	StartOver() string
	ClickImage() string
	ErrTooFewPoints(min int) string
	ErrPointsDontFit() string
	ErrRefine(err error) string
	AutoTap() string
	AutoTapHint() string
	ByHand() string
	ErrAutoFailed() string

	// Checking the grid.
	CheckTitle(name string) string
	CheckHeading(name string) string
	GridWrongTitle() string
	GridWrong(found, searched int) string
	GridWrongHint() string
	CheckExplanation() string
	FurtherOff() string
	Discarded() string
	MeanDeviation(px string) string
	OnlyACheck() string
	FoundFeatures() string

	// Adding cameras.
	DevicesHint() string
	TakeNewImages() string
	WhichIsWhich() string
	NoDevices() string
	ImageUnavailable(name string) string
	OnBoardAs(name string) string
	AddDevice() string
	ErrAddCamera(err string) string

	// Settings.
	Saved() string
	ServerHint() string
	Server() string
	CameraToken() string
	FrameSize() string
	FrameSizeHint() string
	FPS() string
	FPSHint() string
	Advanced() string
	AdvancedHint() string
	Defaults() string
	Save() string
	ErrInvalidSettings(err string) string
	LengthToWidth() string
	Frames() string
	DiffThreshold() string
	DiffThresholdHint() string
	WeakThreshold() string
	WeakThresholdHint() string
	Blur() string
	BlurHint() string
	MinArea() string
	MinAreaHint() string
	MinSpeck() string
	MinSpeckHint() string
	MinElongation() string
	MinElongationHint() string
	Gap() string
	GapHint() string
	AxisDistance() string
	AxisDistanceHint() string
	TipShift() string
	TipShiftHint() string
	SampleStep() string
	SampleStepHint() string
	MotionThreshold() string
	MotionThresholdHint() string
	MotionArea() string
	MotionAreaHint() string
	StableFrames() string
	StableFramesHint() string
	TakeoutArea() string
	TakeoutAreaHint() string
}

// T returns the texts in the language of the request (see i18n.From).
func T(ctx context.Context) Texts {
	return textsOf(i18n.From(ctx))
}

func textsOf(l i18n.Lang) Texts {
	if l == i18n.German {
		return textsDE{}
	}
	return textsEN{}
}

// dartLabel is how a detected dart is shown: 20, D20, T20, Bull, Bullseye, M7 (a miss
// beside the 7), or "Miss" when unknown where.
func dartLabel(t Texts, d throw.Dart) string {
	if d.Segment == throw.Miss && d.Number == 0 {
		return t.Miss()
	}
	return d.String()
}

// legendItem is a color of the trace images and what it means.
type legendItem struct{ Color, Text string }

func traceLegend(t Texts) []legendItem {
	hex := func(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
	return []legendItem{
		{hex(vision.TraceSeed), t.LegendSeed()},
		{hex(vision.TraceArea), t.LegendArea()},
		{hex(vision.TracePiece), t.LegendPiece()},
		{hex(vision.TraceChanged), t.LegendChanged()},
		{hex(vision.TraceEarlier), t.LegendEarlier()},
		{hex(vision.TraceFirst), t.LegendFirstAxis()},
		{hex(vision.TraceAxis), t.LegendAxis()},
		{hex(vision.TraceTipEnd), t.LegendTipEnd()},
		{hex(vision.TraceTipColor), t.LegendTip()},
	}
}
