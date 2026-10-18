package camui

import "strconv"

// textsEN are the English texts, the default language: complete, so every other
// language falls back to them.
type textsEN struct{}

func (textsEN) Number(f float64, digits int) string { return strconv.FormatFloat(f, 'f', digits, 64) }

func (textsEN) NavCameras() string               { return "Cameras" }
func (textsEN) NavAddCamera() string             { return "Add camera" }
func (textsEN) NavSettings() string              { return "Settings" }
func (textsEN) NewVersion(version string) string { return "Version " + version + " available" }
func (textsEN) Back() string                     { return "Back" }
func (textsEN) DetectionRunning(cameras int) string {
	if cameras == 1 {
		return "Detection is running with 1 camera."
	}
	return "Detection is running with " + strconv.Itoa(cameras) + " cameras."
}
func (textsEN) DetectionWaiting() string           { return "Detection is waiting for a calibrated camera." }
func (textsEN) DetectionStopped(err string) string { return "Detection stopped: " + err }
func (textsEN) DetectionOff() string               { return "Detection is turned off." }

func (textsEN) TurnDetectionOff() string { return "Turn detection off" }
func (textsEN) TurnDetectionOn() string  { return "Turn detection on" }
func (textsEN) ThrowsGoTo() string       { return "Throws go to" }
func (textsEN) NotConnectedBefore() string {
	return "Not connected to a server: detected throws only show up here. Under "
}
func (textsEN) NotConnectedAfter() string {
	return " enter the darts-o-mat server and the board's camera token."
}
func (textsEN) NoCameras() string {
	return "No cameras yet. Add your board's cameras under “Add camera” and calibrate them with an empty board."
}
func (textsEN) CalibrateHintBefore() string { return "Calibrate each camera once with an" }
func (textsEN) EmptyBoard() string          { return "empty board" }
func (textsEN) CalibrateHintAfter() string {
	return "; then the board's grid lies on its image and the detection uses it right away."
}
func (textsEN) RefreshImages() string      { return "Refresh images" }
func (textsEN) Reconnect() string          { return "Reconnect cameras" }
func (textsEN) ReconnectHint() string      { return "After plugging a camera in or out." }
func (textsEN) ImageOf(name string) string { return "Image of " + name }
func (textsEN) PixelsPerMM() string        { return "Pixels per millimetre on the board" }
func (textsEN) Calibrated() string         { return "calibrated" }
func (textsEN) NotCalibrated() string      { return "not calibrated" }
func (textsEN) CalibratedForOtherSize(size string) string {
	return "Calibrated for " + size + ", but the camera delivers another size now: please calibrate again."
}
func (textsEN) Recalibrate() string      { return "Calibrate again" }
func (textsEN) Calibrate() string        { return "Calibrate" }
func (textsEN) CheckGrid() string        { return "Check the grid" }
func (textsEN) RefineOnly() string       { return "Fine calibration only" }
func (textsEN) Remove() string           { return "Remove" }
func (textsEN) LatestDetections() string { return "Latest detections" }
func (textsEN) ConfigHint(dir string) string {
	return "Calibration and settings are kept on this computer in " + dir + " and work with any darts-o-mat server."
}
func (textsEN) ProblemNotConnected() string { return "Not connected." }
func (textsEN) ProblemMoved() string {
	return "The image doesn't match the one of the calibration anymore (camera moved?). Please calibrate again."
}

func (textsEN) NoEvents() string {
	return "None yet. They show up here as soon as the detection sees throws."
}
func (textsEN) DartsPulledOut() string { return "Darts pulled out" }
func (textsEN) NCameras(n int) string {
	return "(" + strconv.Itoa(n) + " " + plural(n, "camera", "cameras") + ")"
}
func (textsEN) NotReported(err string) string     { return "not entered: " + err }
func (textsEN) ReportedInGame(game string) string { return "entered in game " + game }
func (textsEN) Miss() string                      { return "Miss" }
func (textsEN) DetectionTitle() string            { return "Detection" }
func (textsEN) UnlocatedNotice() string {
	return "Something changed and no darts were pulled out, but no camera found the dart on the board: counted as a miss, without a position."
}
func (textsEN) Camera() string           { return "Camera" }
func (textsEN) PositionMM() string       { return "Position (mm)" }
func (textsEN) DistanceToCenter() string { return "Distance to the center" }
func (textsEN) Resolution() string       { return "Resolution (px/mm)" }
func (textsEN) Field() string            { return "Field" }
func (textsEN) Hidden() string           { return "(hidden)" }
func (textsEN) Result() string           { return "✕ Result" }
func (textsEN) EventExplanation() string {
	return "Boundaries: triple 99–107 mm, double 162–170 mm, bull up to 15.9 mm, bullseye up to 6.35 mm. Resolution: pixels per mm at this spot, towards the center / along the ring. The result combines the distances to the center and the angles separately, each camera counting with its resolution in that direction: from three cameras the weighted median, with two the weighted mean (near the center per coordinate). If a camera's tip ends on an earlier dart (“hidden”), it only counts if no other camera sees the dart."
}
func (textsEN) Before() string { return "Before" }
func (textsEN) After() string  { return "After" }

func (textsEN) RemoveTitle(name string) string    { return "Remove " + name }
func (textsEN) RemoveQuestion(name string) string { return "Remove " + name + "?" }
func (textsEN) RemoveHint() string {
	return "The camera is removed from the board, together with its calibration. You can add it again later and calibrate it anew."
}
func (textsEN) Cancel() string { return "Cancel" }

func (textsEN) CalibrateTitle(name string) string   { return "Calibrate: " + name }
func (textsEN) CalibrateHeading(name string) string { return "Calibrate " + name }
func (textsEN) ClickBefore() string                 { return "Click where the wire between" }
func (textsEN) And() string                         { return "and" }
func (textsEN) ClickAfter() string                  { return "crosses the outer edge of the double ring." }
func (textsEN) ClickHint() string {
	return "Roughly is enough, the fine calibration evens it out. If the spot is hidden or not in the image, skip it."
}
func (textsEN) AllPointsDone() string { return "All points done." }
func (textsEN) PointsSet(points, min int) string {
	return strconv.Itoa(points) + " " + plural(points, "point", "points") + " set, at least " + strconv.Itoa(min) + " needed."
}
func (textsEN) Skip() string              { return "Skip" }
func (textsEN) FinishCalibration() string { return "Done: fine-calibrate and save" }
func (textsEN) StartOver() string         { return "Start over (new image)" }
func (textsEN) ClickImage() string        { return "Camera image, click here" }
func (textsEN) ErrTooFewPoints(min int) string {
	return "At least " + strconv.Itoa(min) + " points needed."
}
func (textsEN) ErrPointsDontFit() string { return "The points don't fit together, please start over." }
func (textsEN) ErrRefine(err error) string {
	return "Fine calibration failed (" + err.Error() + "); is the board empty and well visible?"
}

func (textsEN) CheckTitle(name string) string   { return "Check the grid: " + name }
func (textsEN) CheckHeading(name string) string { return "Check the grid of " + name }
func (textsEN) GridWrongTitle() string          { return "The grid doesn't fit the board:" }
func (textsEN) GridWrong(found, searched int) string {
	return "only " + strconv.Itoa(found) + " of " + strconv.Itoa(searched) + " edges and wires are where the grid expects them."
}
func (textsEN) GridWrongHint() string {
	return "Usually a wire was mixed up while calibrating, e.g. 1/18 instead of 20/1. Check the numbers closely (they're upside down for some cameras) and calibrate again. Cross-check with the colors: the double of the 20 is red, that of the 1 green."
}
func (textsEN) CheckExplanation() string {
	return "Searched on the current image: the edges of the colored rings and the wires between the numbers. Blue is the grid as the calibration sees the board; the dots are the edges and wires found. If the blue lines lie on the real wires and nearly all dots are green, the calibration is right."
}
func (textsEN) FurtherOff() string             { return "further off" }
func (textsEN) Discarded() string              { return "discarded (e.g. a dart in front)" }
func (textsEN) MeanDeviation(px string) string { return "mean deviation " + px + " px" }
func (textsEN) OnlyACheck() string {
	return "This is only a check, nothing is saved. Darts in the board show discarded points at them; that's fine."
}
func (textsEN) FoundFeatures() string { return "Edges and wires found" }

func (textsEN) DevicesHint() string {
	return "All cameras on this computer. Add the ones looking at the board; then it gets calibrated. darts-o-cam tells identical cameras apart by their image of the board, not by the USB port: plugging them into other ports is no problem."
}
func (textsEN) TakeNewImages() string { return "Take new images" }
func (textsEN) WhichIsWhich() string {
	return "Which is which? Hold a hand in front of a camera and take new images."
}
func (textsEN) NoDevices() string {
	return "No cameras found. Is the camera connected, and may darts-o-cam use it?"
}
func (textsEN) ImageUnavailable(name string) string { return "Image of " + name + " (not available)" }
func (textsEN) OnBoardAs(name string) string        { return "on the board as " + name }
func (textsEN) AddDevice() string                   { return "Add" }
func (textsEN) ErrAddCamera(err string) string      { return "The camera couldn't be added: " + err }

func (textsEN) Saved() string { return "Saved and applied." }
func (textsEN) ServerHint() string {
	return "Create your board on the server under “Boards” and create a camera token. Detected throws then go into the running game on this board."
}
func (textsEN) Server() string      { return "Server" }
func (textsEN) CameraToken() string { return "Camera token" }
func (textsEN) FrameSize() string   { return "Wanted frame size" }
func (textsEN) FrameSizeHint() string {
	return "Each camera takes its own mode closest to this size; the size it delivers is on its card. The cameras are opened again for it. A calibration only holds for the size it was made in."
}
func (textsEN) FPS() string { return "Frames per second" }
func (textsEN) FPSHint() string {
	return "The detection looks at this many frames per second; more reacts faster but needs more computing power."
}
func (textsEN) Advanced() string { return "Detection (advanced)" }
func (textsEN) AdvancedHint() string {
	return "Sizes are in millimetres on the board; darts-o-cam converts them to pixels for each camera by its resolution and distance. The defaults come from a board with three cameras at 1280 × 720."
}
func (textsEN) Defaults() string                     { return "Defaults" }
func (textsEN) Save() string                         { return "Save" }
func (textsEN) ErrInvalidSettings(err string) string { return "Invalid settings: " + err }
func (textsEN) LengthToWidth() string                { return "length : width" }
func (textsEN) Frames() string                       { return "frames" }
func (textsEN) DiffThreshold() string                { return "Color difference" }
func (textsEN) DiffThresholdHint() string {
	return "From this difference on, a pixel counts as changed."
}
func (textsEN) WeakThreshold() string { return "Weak color difference" }
func (textsEN) WeakThresholdHint() string {
	return "Only counts next to strongly changed pixels; keeps the thin point at the dart."
}
func (textsEN) Blur() string          { return "Blur" }
func (textsEN) BlurHint() string      { return "Radius against image noise." }
func (textsEN) MinArea() string       { return "Minimum dart size" }
func (textsEN) MinAreaHint() string   { return "Smaller changes aren't a dart." }
func (textsEN) MinSpeck() string      { return "Smallest speck" }
func (textsEN) MinSpeckHint() string  { return "Smaller specks are ignored as noise." }
func (textsEN) MinElongation() string { return "Minimum elongation" }
func (textsEN) MinElongationHint() string {
	return "This elongated the lower part of the dart (or the whole piece) has to be."
}
func (textsEN) Gap() string { return "Bridge gaps" }
func (textsEN) GapHint() string {
	return "Pieces with gaps up to twice this count together (dashed barrel in front of black fields)."
}
func (textsEN) AxisDistance() string { return "Distance to the axis" }
func (textsEN) AxisDistanceHint() string {
	return "Pieces this close to the dart's axis belong to the dart (e.g. the flight)."
}
func (textsEN) TipShift() string { return "Move the tip back" }
func (textsEN) TipShiftHint() string {
	return "The tip found is moved back this far along the dart: the change in the image usually reaches a little beyond the real tip (shadow)."
}
func (textsEN) SampleStep() string { return "Motion: distance of the samples" }
func (textsEN) SampleStepHint() string {
	return "For detecting motion, only pixels this far apart are compared."
}
func (textsEN) MotionThreshold() string { return "Motion: color difference" }
func (textsEN) MotionThresholdHint() string {
	return "From this difference on, a sample counts as moved."
}
func (textsEN) MotionArea() string { return "Motion from" }
func (textsEN) MotionAreaHint() string {
	return "This much has to change for the board to count as moving."
}
func (textsEN) StableFrames() string { return "Still frames" }
func (textsEN) StableFramesHint() string {
	return "This many frames without motion before the change is evaluated."
}
func (textsEN) TakeoutArea() string { return "Takeout from" }
func (textsEN) TakeoutAreaHint() string {
	return "If more changes, it was a hand or darts pulled out."
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (textsEN) TraceTitle() string { return "Detection in detail" }
func (textsEN) TraceHint() string {
	return "Computed again from the saved frames, with the current calibration and settings; the earlier darts of the turn are detected again too. Click an image to see it in full size."
}
func (textsEN) LegendSeed() string      { return "barrel and point" }
func (textsEN) LegendArea() string      { return "rest of the dart (e.g. flight)" }
func (textsEN) LegendPiece() string     { return "changed piece, not the dart" }
func (textsEN) LegendChanged() string   { return "changed speck" }
func (textsEN) LegendEarlier() string   { return "earlier darts" }
func (textsEN) LegendFirstAxis() string { return "axis of the lower half" }
func (textsEN) LegendAxis() string      { return "axis near the tip" }
func (textsEN) LegendTipEnd() string    { return "end the tip is averaged from" }
func (textsEN) LegendTip() string       { return "tip" }
func (textsEN) TraceChanged(pixels, pieces int) string {
	return strconv.Itoa(pixels) + " pixels changed, " + strconv.Itoa(pieces) + " " + plural(pieces, "piece", "pieces")
}
func (textsEN) TraceDart(seed, area int, elongation string) string {
	return "barrel and point " + strconv.Itoa(seed) + " px (length : width " + elongation + "), whole dart " + strconv.Itoa(area) + " px"
}
func (textsEN) TraceRetried() string {
	return "not shaped like a dart at first: computed again without the earlier darts"
}
func (textsEN) TraceNoDart(err string) string { return "no dart: " + err }
func (textsEN) Zoom() string                  { return "Zoom" }
func (textsEN) WholeImage() string            { return "Whole image" }
