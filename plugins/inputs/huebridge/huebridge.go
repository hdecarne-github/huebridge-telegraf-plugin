// huebridge.go
//
// Copyright (C) 2022 Holger de Carne
//
// This software may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.

package huebridge

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	"golang.org/x/exp/slices"
)

type HueBridge struct {
	Bridges         [][]string `toml:"bridges"`
	Timeout         int        `toml:"timeout"`
	RoomAssignments [][]string `toml:"room_assignments"`
	Debug           bool       `toml:"debug"`

	Log telegraf.Logger

	cachedClient *http.Client
}

func NewHueBridge() *HueBridge {
	return &HueBridge{
		Bridges: [][]string{},
		Timeout: 10,
	}
}

func (plugin *HueBridge) SampleConfig() string {
	return `
  ## The Hue bridges to query (multiple tuples of base url, application key)
  ## To create a application key issue the following command line for the targeted Hue bridge:
  ## curl -X POST http://<bridge IP or DNS name>/api -H 'Content-Type: application/json' -d '{"devicetype":"huebridge-telegraf-plugin"}'
  bridges = [["https://<insert IP or DNS name>", "<insert application key>"]]
  ## The http timeout to use (in seconds)
  # timeout = 10
  ## In case a device cannot be assigned to a room (e.g. a motion sensor), the following option
  ## allows a manual assignment. Every sub-array defines an assignment. The 1st element names
  ## the room and the following elements the devices to assign to this room.
  # room_assignments = [["room", "device 1"]]
  ## Enable debug output
  # debug = false
 `
}

func (plugin *HueBridge) Description() string {
	return "Gather Hue Bridge status"
}

func (plugin *HueBridge) Gather(a telegraf.Accumulator) error {
	if len(plugin.Bridges) == 0 {
		return errors.New("huebridge: Empty bridge list")
	}
	for _, bridge := range plugin.Bridges {
		if len(bridge) != 2 {
			return fmt.Errorf("huebridge: Invalid bridge entry: %s", bridge)
		}
		bridgeUrl := bridge[0]
		username := bridge[1]
		a.AddError(plugin.processBridge(a, bridgeUrl, username))
	}
	return nil
}

func (plugin *HueBridge) processBridge(a telegraf.Accumulator, bridgeUrl string, applicationKey string) error {
	if plugin.Debug {
		plugin.Log.Infof("Processing bridge: %s", bridgeUrl)
	}
	devices, err := plugin.fetchDevices(a, bridgeUrl, applicationKey)
	if err != nil {
		return err
	}
	rooms, err := plugin.fetchRooms(a, bridgeUrl, applicationKey)
	if err != nil {
		return err
	}
	lights, err := plugin.fetchLights(a, bridgeUrl, applicationKey)
	if err == nil {
		plugin.evalLights(a, bridgeUrl, lights, devices, rooms)
	} else {
		a.AddError(fmt.Errorf("failed to eval lights (cause: %w)", err))
	}
	temperatures, err := plugin.fetchTemperatures(a, bridgeUrl, applicationKey)
	if err == nil {
		plugin.evalTemperatures(a, bridgeUrl, temperatures, devices, rooms)
	} else {
		a.AddError(fmt.Errorf("failed to eval temperatures (cause: %w)", err))
	}
	lightLevels, err := plugin.fetchLightLevels(a, bridgeUrl, applicationKey)
	if err == nil {
		plugin.evalLightLevels(a, bridgeUrl, lightLevels, devices, rooms)
	} else {
		a.AddError(fmt.Errorf("failed to eval light levels (cause: %w)", err))
	}
	motions, err := plugin.fetchMotions(a, bridgeUrl, applicationKey)
	if err == nil {
		plugin.evalMotions(a, bridgeUrl, motions, devices, rooms)
	} else {
		a.AddError(fmt.Errorf("failed to eval motions (cause: %w)", err))
	}
	devicePowers, err := plugin.fetchDevicePowers(a, bridgeUrl, applicationKey)
	if err == nil {
		plugin.evalDevicePowers(a, bridgeUrl, devicePowers, devices)
	} else {
		a.AddError(fmt.Errorf("failed to eval motions (cause: %w)", err))
	}
	return nil
}

func (plugin *HueBridge) evalLights(a telegraf.Accumulator, bridgeUrl string, lights *lightsStatus, devices *devicesList, rooms *roomsList) {
	for _, light := range lights.Data {
		lightDeviceName, lightRoomName := light.Owner.getDeviceAndRoomName(devices, rooms, plugin.RoomAssignments)
		tags := make(map[string]string)
		tags["huebridge_url"] = bridgeUrl
		tags["huebridge_room"] = lightRoomName
		tags["huebridge_device"] = lightDeviceName
		fields := make(map[string]interface{})
		if light.On.On {
			fields["on"] = 1
		} else {
			fields["on"] = 0
		}
		a.AddCounter("huebridge_light", fields, tags)
	}
}

func (plugin *HueBridge) evalTemperatures(a telegraf.Accumulator, bridgeUrl string, temperatures *temperaturesStatus, devices *devicesList, rooms *roomsList) {
	for _, temperature := range temperatures.Data {
		if temperature.Enabled && temperature.Temperature.TemperatureValid {
			temperatureDeviceName, temperatureRoomName := temperature.Owner.getDeviceAndRoomName(devices, rooms, plugin.RoomAssignments)
			tags := make(map[string]string)
			tags["huebridge_url"] = bridgeUrl
			tags["huebridge_room"] = temperatureRoomName
			tags["huebridge_device"] = temperatureDeviceName
			fields := make(map[string]interface{})
			fields["temperature"] = temperature.Temperature.Temperature
			a.AddCounter("huebridge_temperature", fields, tags)
		}
	}
}

func (plugin *HueBridge) evalLightLevels(a telegraf.Accumulator, bridgeUrl string, lightLevels *lightLevelsStatus, devices *devicesList, rooms *roomsList) {
	for _, lightLevel := range lightLevels.Data {
		if lightLevel.Enabled && lightLevel.Light.LightLevelValid {
			lightLevelDeviceName, lightLevelRoomName := lightLevel.Owner.getDeviceAndRoomName(devices, rooms, plugin.RoomAssignments)
			tags := make(map[string]string)
			tags["huebridge_url"] = bridgeUrl
			tags["huebridge_room"] = lightLevelRoomName
			tags["huebridge_device"] = lightLevelDeviceName
			fields := make(map[string]interface{})
			fields["light_level"] = lightLevel.Light.LightLevel
			fields["light_level_lux"] = math.Pow(10.0, (float64(lightLevel.Light.LightLevel)-1.0)/10000.0)
			a.AddCounter("huebridge_light_level", fields, tags)
		}
	}
}

func (plugin *HueBridge) evalMotions(a telegraf.Accumulator, bridgeUrl string, motions *motionsStatus, devices *devicesList, rooms *roomsList) {
	for _, motion := range motions.Data {
		if motion.Enabled && motion.Motion.MotionValid {
			motionDeviceName, motionRoomName := motion.Owner.getDeviceAndRoomName(devices, rooms, plugin.RoomAssignments)
			tags := make(map[string]string)
			tags["huebridge_url"] = bridgeUrl
			tags["huebridge_room"] = motionRoomName
			tags["huebridge_device"] = motionDeviceName
			fields := make(map[string]interface{})
			if motion.Motion.Motion {
				fields["motion"] = 1
			} else {
				fields["motion"] = 0
			}
			a.AddCounter("huebridge_motion", fields, tags)
		}
	}
}

func (plugin *HueBridge) evalDevicePowers(a telegraf.Accumulator, bridgeUrl string, devicePowers *devicePowersStatus, devices *devicesList) {
	for _, devicePower := range devicePowers.Data {
		devicePowerDeviceName := devicePower.Owner.getDeviceName(devices)
		tags := make(map[string]string)
		tags["huebridge_url"] = bridgeUrl
		tags["huebridge_device"] = devicePowerDeviceName
		fields := make(map[string]interface{})
		fields["battery_level"] = devicePower.PowerState.BatteryLevel
		a.AddCounter("huebridge_device_power", fields, tags)
	}
}

type lightsStatus struct {
	Data []lightData `json:"data"`
}

type lightData struct {
	On    lightOn      `json:"on"`
	Owner resourceLink `json:"owner"`
}

type lightOn struct {
	On bool `json:"on"`
}

type temperaturesStatus struct {
	Data []temperatureData `json:"data"`
}

type temperatureData struct {
	Enabled     bool                   `json:"enabled"`
	Temperature temperatureTemperature `json:"temperature"`
	Owner       resourceLink           `json:"owner"`
}

type temperatureTemperature struct {
	Temperature      float32 `json:"temperature"`
	TemperatureValid bool    `json:"temperature_valid"`
}

type lightLevelsStatus struct {
	Data []lightLevelData `json:"data"`
}

type lightLevelData struct {
	Enabled bool            `json:"enabled"`
	Light   lightLevelLight `json:"light"`
	Owner   resourceLink    `json:"owner"`
}

type lightLevelLight struct {
	LightLevel      float32 `json:"light_level"`
	LightLevelValid bool    `json:"light_level_valid"`
}

type motionsStatus struct {
	Data []motionData `json:"data"`
}

type motionData struct {
	Enabled bool         `json:"enabled"`
	Motion  motionMotion `json:"motion"`
	Owner   resourceLink `json:"owner"`
}

type motionMotion struct {
	Motion      bool `json:"motion"`
	MotionValid bool `json:"motion_valid"`
}

type devicePowersStatus struct {
	Data []devicePowerData `json:"data"`
}

type devicePowerData struct {
	PowerState devicePowerState `json:"power_state"`
	Owner      resourceLink     `json:"owner"`
}

type devicePowerState struct {
	BatteryState string `json:"battery_state"`
	BatteryLevel int    `json:"battery_level"`
}

type devicesList struct {
	Data []deviceData `json:"data"`
}

func (ds *devicesList) findDeviceData(deviceId string) *deviceData {
	for _, device := range ds.Data {
		if device.Id == deviceId {
			return &device
		}
	}
	return nil
}

type deviceData struct {
	Id       string           `json:"id"`
	Metadata resourceMetadata `json:"metadata"`
}

type roomsList struct {
	Data []roomData `json:"data"`
}

func (rs *roomsList) findDeviceRoomData(childDeviceId string) *roomData {
	for _, room := range rs.Data {
		for _, child := range room.Children {
			if child.Rtype == "device" && child.Rid == childDeviceId {
				return &room
			}
		}
	}
	return nil
}

type roomData struct {
	Id       string           `json:"id"`
	Metadata resourceMetadata `json:"metadata"`
	Children []resourceLink   `json:"children"`
}

type resourceMetadata struct {
	Archetype string `json:"archetype"`
	Name      string `json:"name"`
}

type resourceLink struct {
	Rid   string `json:"rid"`
	Rtype string `json:"rtype"`
}

const undefinedDevice = "<undefined>"
const unassignedDevice = "<unassigned>"

func (rl *resourceLink) getDeviceName(devices *devicesList) string {
	deviceName := undefinedDevice
	if rl.Rtype == "device" {
		device := devices.findDeviceData(rl.Rid)
		if device != nil {
			deviceName = device.Metadata.Name
		}
	}
	return deviceName
}

func (rl *resourceLink) getDeviceAndRoomName(devices *devicesList, rooms *roomsList, roomAssignments [][]string) (string, string) {
	deviceName := undefinedDevice
	roomName := unassignedDevice
	if rl.Rtype == "device" {
		device := devices.findDeviceData(rl.Rid)
		if device != nil {
			deviceName = device.Metadata.Name
			for _, assignment := range roomAssignments {
				index := slices.Index(assignment, deviceName)
				if index > 0 {
					roomName = assignment[0]
					break
				}
			}
			if roomName == unassignedDevice {
				room := rooms.findDeviceRoomData(device.Id)
				if room != nil {
					roomName = room.Metadata.Name
				}
			}
		}
	}
	return deviceName, roomName
}

func (plugin *HueBridge) fetchLights(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*lightsStatus, error) {
	var lightsStatus lightsStatus

	_, err := plugin.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/light", &lightsStatus)
	if err != nil {
		return nil, err
	}
	return &lightsStatus, nil
}

func (plugin *HueBridge) fetchTemperatures(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*temperaturesStatus, error) {
	var temperaturesStatus temperaturesStatus

	_, err := plugin.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/temperature", &temperaturesStatus)
	if err != nil {
		return nil, err
	}
	return &temperaturesStatus, nil
}

func (plugin *HueBridge) fetchLightLevels(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*lightLevelsStatus, error) {
	var lightLevelsStatus lightLevelsStatus

	_, err := plugin.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/light_level", &lightLevelsStatus)
	if err != nil {
		return nil, err
	}
	return &lightLevelsStatus, nil
}

func (plugin *HueBridge) fetchMotions(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*motionsStatus, error) {
	var motionsStatus motionsStatus

	_, err := plugin.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/motion", &motionsStatus)
	if err != nil {
		return nil, err
	}
	return &motionsStatus, nil
}

func (plugin *HueBridge) fetchDevicePowers(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*devicePowersStatus, error) {
	var devicePowersStatus devicePowersStatus

	_, err := plugin.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/device_power", &devicePowersStatus)
	if err != nil {
		return nil, err
	}
	return &devicePowersStatus, nil
}

func (plugin *HueBridge) fetchDevices(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*devicesList, error) {
	var devicesList devicesList

	_, err := plugin.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/device", &devicesList)
	if err != nil {
		return nil, err
	}
	return &devicesList, nil
}

func (plugin *HueBridge) fetchRooms(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*roomsList, error) {
	var roomsList roomsList

	_, err := plugin.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/room", &roomsList)
	if err != nil {
		return nil, err
	}
	return &roomsList, nil
}

func (plugin *HueBridge) fetchJSON(bridgeUrl string, applicationKey string, path string, v interface{}) (*url.URL, error) {
	baseUrl, err := url.Parse(bridgeUrl)
	if err != nil {
		return nil, err
	}
	pathUrl, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	jsonUrl := baseUrl.ResolveReference(pathUrl)
	if plugin.Debug {
		plugin.Log.Infof("Fetching JSON from: %s", jsonUrl)
	}
	request, err := http.NewRequest("GET", jsonUrl.String(), nil)
	if err != nil {
		return jsonUrl, err
	}
	request.Header.Add("hue-application-key", applicationKey)
	client := plugin.getClient()
	response, err := client.Do(request)
	if err != nil {
		return jsonUrl, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return jsonUrl, fmt.Errorf("failed to retrieve json data from %s (%s)", jsonUrl, response.Status)
	}
	return jsonUrl, json.NewDecoder(response.Body).Decode(v)
}

func (plugin *HueBridge) getClient() *http.Client {
	if plugin.cachedClient == nil {
		transport := &http.Transport{
			ResponseHeaderTimeout: time.Duration(plugin.Timeout) * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		}
		plugin.cachedClient = &http.Client{
			Transport: transport,
			Timeout:   time.Duration(plugin.Timeout) * time.Second,
		}
	}
	return plugin.cachedClient
}

func init() {
	inputs.Add("huebridge", func() telegraf.Input {
		return NewHueBridge()
	})
}
