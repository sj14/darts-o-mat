package vision

import "github.com/sj14/darts-o-mat/pkg/camera"

// DeviceGroup is a kind of camera that is connected: all video devices with the same
// name. A board's cameras are usually identical, so they form one group.
//
// Single cameras of a group can't be told apart by the hardware: the development board's
// cameras even share their serial number, and the ids macOS and Linux offer depend on the
// USB port. Which camera is which is told by its view instead (see AssignCameras).
type DeviceGroup struct {
	Name  string `json:"name"`
	Model string `json:"model,omitempty"` // e.g. "1e45:8022" (USB vendor:product), if known
	Count int    `json:"count"`
}

// ListDeviceGroups returns the connected video devices grouped by name.
func ListDeviceGroups() ([]DeviceGroup, error) {
	devices, err := camera.Devices()
	if err != nil {
		return nil, err
	}
	var groups []DeviceGroup
	index := map[string]int{}
	for _, d := range devices {
		i, ok := index[d.Name]
		if !ok {
			i = len(groups)
			index[d.Name] = i
			groups = append(groups, DeviceGroup{Name: d.Name, Model: d.Model})
		}
		groups[i].Count++
	}
	return groups, nil
}
