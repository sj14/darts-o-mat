package vision

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

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

// ListDeviceGroups returns the connected video devices grouped by name (macOS).
// Screen captures are left out.
func ListDeviceGroups() ([]DeviceGroup, error) {
	devices, err := ListAVFoundationDevices()
	if err != nil {
		return nil, err
	}
	models, _ := cameraModels() // only for showing them, so errors don't matter

	var groups []DeviceGroup
	index := map[string]int{}
	for _, d := range devices {
		if strings.HasPrefix(d.Name, "Capture screen") {
			continue
		}
		i, ok := index[d.Name]
		if !ok {
			i = len(groups)
			index[d.Name] = i
			groups = append(groups, DeviceGroup{Name: d.Name, Model: models[d.Name]})
		}
		groups[i].Count++
	}
	return groups, nil
}

var modelPattern = regexp.MustCompile(`VendorID_(\d+) ProductID_(\d+)`)

// cameraModels returns the USB vendor:product id of the cameras by name, from system_profiler (macOS).
func cameraModels() (map[string]string, error) {
	out, err := exec.Command("system_profiler", "SPCameraDataType", "-json").Output()
	if err != nil {
		return nil, err
	}
	var data struct {
		Cameras []struct {
			Name  string `json:"_name"`
			Model string `json:"spcamera_model-id"`
		} `json:"SPCameraDataType"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, err
	}
	models := map[string]string{}
	for _, c := range data.Cameras {
		if m := modelPattern.FindStringSubmatch(c.Model); m != nil {
			vendor, _ := strconv.Atoi(m[1])
			product, _ := strconv.Atoi(m[2])
			models[c.Name] = fmt.Sprintf("%04x:%04x", vendor, product)
		}
	}
	return models, nil
}
