// huebridge.go
//
// Copyright (C) 2022 Holger de Carne
//
// This software may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.
//
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
)

const undefined = "<undefined>"

var plugin = undefined
var version = undefined
var goos = undefined
var goarch = undefined

type HueBridge struct {
	Bridges [][]string `toml:"bridges"`
	Timeout int        `toml:"timeout"`
	Debug   bool       `toml:"debug"`

	Log telegraf.Logger

	cachedClient *http.Client
}

func NewHueBridge() *HueBridge {
	return &HueBridge{
		Bridges: [][]string{},
		Timeout: 5}
}

func (hb *HueBridge) SampleConfig() string {
	return `
  [[inputs.huebridge]]
  ## The Hue bridges to query (multiple tuples of base url, username)
  ## To create a username issue the following command line for the targeted Hue bridge:
  ## curl -X POST http://<bridge IP or DNS name>/api -H 'Content-Type: application/json' -d '{"devicetype":"huebridge-telegraf-plugin"}'
  bridges = [["http://192.168.1.2", ""]]
  ## The http timeout to use (in seconds)
  # timeout = 5
  ## Enable debug output
  # debug = false
 `
}

func (hb *HueBridge) Description() string {
	return "Gather Hue Bridge status"
}

func (hb *HueBridge) Gather(a telegraf.Accumulator) error {
	if len(hb.Bridges) == 0 {
		return errors.New("huebridge: Empty bridge list")
	}
	for _, bridge := range hb.Bridges {
		if len(bridge) != 2 {
			return fmt.Errorf("huebridge: Invalid bridge entry: %s", bridge)
		}
		bridgeUrl := bridge[0]
		username := bridge[1]
		a.AddError(hb.processBridge(a, bridgeUrl, username))
	}
	return nil
}

func (hb *HueBridge) processBridge(a telegraf.Accumulator, bridgeUrl string, applicationKey string) error {
	if hb.Debug {
		hb.Log.Infof("Processing bridge: %s", bridgeUrl)
	}
	devices, err := hb.fetchDevices(a, bridgeUrl, applicationKey)
	if err != nil {
		return err
	}
	rooms, err := hb.fetchRooms(a, bridgeUrl, applicationKey)
	if err != nil {
		return err
	}
	lights, err := hb.fetchLights(a, bridgeUrl, applicationKey)
	if err == nil {
		hb.evalLights(a, bridgeUrl, lights, devices, rooms)
	} else {
		a.AddError(fmt.Errorf("Failed to eval lights (cause: %w)", err))
	}
	temperatures, err := hb.fetchTemperatures(a, bridgeUrl, applicationKey)
	if err == nil {
		hb.evalTemperatures(a, bridgeUrl, temperatures, devices)
	} else {
		a.AddError(fmt.Errorf("Failed to eval temperatures (cause: %w)", err))
	}
	lightLevels, err := hb.fetchLightLevels(a, bridgeUrl, applicationKey)
	if err == nil {
		hb.evalLightLevels(a, bridgeUrl, lightLevels, devices)
	} else {
		a.AddError(fmt.Errorf("Failed to eval light levels (cause: %w)", err))
	}
	motions, err := hb.fetchMotions(a, bridgeUrl, applicationKey)
	if err == nil {
		hb.evalMotions(a, bridgeUrl, motions, devices)
	} else {
		a.AddError(fmt.Errorf("Failed to eval motions (cause: %w)", err))
	}
	return nil
}

func (hb *HueBridge) evalLights(a telegraf.Accumulator, bridgeUrl string, lights *lightsStatus, devices *devicesList, rooms *roomsList) {
	for _, light := range lights.Data {
		lightDeviceName, lightRoomName := light.Owner.getDeviceAndRoomName(devices, rooms)
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

func (hb *HueBridge) evalTemperatures(a telegraf.Accumulator, bridgeUrl string, temperatures *temperaturesStatus, devices *devicesList) {
	for _, temperature := range temperatures.Data {
		if temperature.Enabled && temperature.Temperature.TemperatureValid {
			temperatureDeviceName := temperature.Owner.getDeviceName(devices)
			tags := make(map[string]string)
			tags["huebridge_url"] = bridgeUrl
			tags["huebridge_device"] = temperatureDeviceName
			fields := make(map[string]interface{})
			fields["temperature"] = temperature.Temperature.Temperature
			a.AddCounter("huebridge_temperature", fields, tags)
		}
	}
}

func (hb *HueBridge) evalLightLevels(a telegraf.Accumulator, bridgeUrl string, lightLevels *lightLevelsStatus, devices *devicesList) {
	for _, lightLevel := range lightLevels.Data {
		if lightLevel.Enabled && lightLevel.Light.LightLevelValid {
			lightLevelDeviceName := lightLevel.Owner.getDeviceName(devices)
			tags := make(map[string]string)
			tags["huebridge_url"] = bridgeUrl
			tags["huebridge_device"] = lightLevelDeviceName
			fields := make(map[string]interface{})
			fields["light_level"] = lightLevel.Light.LightLevel
			fields["light_level_lux"] = math.Pow(10.0, (float64(lightLevel.Light.LightLevel)-1.0)/10000.0)
			a.AddCounter("huebridge_light_level", fields, tags)
		}
	}
}

func (hb *HueBridge) evalMotions(a telegraf.Accumulator, bridgeUrl string, motions *motionsStatus, devices *devicesList) {
	for _, motion := range motions.Data {
		if motion.Enabled && motion.Motion.MotionValid {
			motionDeviceName := motion.Owner.getDeviceName(devices)
			tags := make(map[string]string)
			tags["huebridge_url"] = bridgeUrl
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

func (rl *resourceLink) getDeviceName(devices *devicesList) string {
	deviceName := "<undefined>"
	if rl.Rtype == "device" {
		device := devices.findDeviceData(rl.Rid)
		if device != nil {
			deviceName = device.Metadata.Name
		}
	}
	return deviceName
}

func (rl *resourceLink) getDeviceAndRoomName(devices *devicesList, rooms *roomsList) (string, string) {
	deviceName := "<undefined>"
	roomName := "<unassigned>"
	if rl.Rtype == "device" {
		device := devices.findDeviceData(rl.Rid)
		if device != nil {
			deviceName = device.Metadata.Name
			room := rooms.findDeviceRoomData(device.Id)
			if room != nil {
				roomName = room.Metadata.Name
			}
		}
	}
	return deviceName, roomName
}

func (hb *HueBridge) fetchLights(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*lightsStatus, error) {
	var lightsStatus lightsStatus

	_, err := hb.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/light", &lightsStatus)
	if err != nil {
		return nil, err
	}
	return &lightsStatus, nil
}

func (hb *HueBridge) fetchTemperatures(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*temperaturesStatus, error) {
	var temperaturesStatus temperaturesStatus

	_, err := hb.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/temperature", &temperaturesStatus)
	if err != nil {
		return nil, err
	}
	return &temperaturesStatus, nil
}

func (hb *HueBridge) fetchLightLevels(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*lightLevelsStatus, error) {
	var lightLevelsStatus lightLevelsStatus

	_, err := hb.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/light_level", &lightLevelsStatus)
	if err != nil {
		return nil, err
	}
	return &lightLevelsStatus, nil
}

func (hb *HueBridge) fetchMotions(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*motionsStatus, error) {
	var motionsStatus motionsStatus

	_, err := hb.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/motion", &motionsStatus)
	if err != nil {
		return nil, err
	}
	return &motionsStatus, nil
}

func (hb *HueBridge) fetchDevices(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*devicesList, error) {
	var devicesList devicesList

	_, err := hb.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/device", &devicesList)
	if err != nil {
		return nil, err
	}
	return &devicesList, nil
}

func (hb *HueBridge) fetchRooms(a telegraf.Accumulator, bridgeUrl string, applicationKey string) (*roomsList, error) {
	var roomsList roomsList

	_, err := hb.fetchJSON(bridgeUrl, applicationKey, "/clip/v2/resource/room", &roomsList)
	if err != nil {
		return nil, err
	}
	return &roomsList, nil
}

func (hb *HueBridge) fetchJSON(bridgeUrl string, applicationKey string, path string, v interface{}) (*url.URL, error) {
	baseUrl, err := url.Parse(bridgeUrl)
	if err != nil {
		return nil, err
	}
	pathUrl, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	jsonUrl := baseUrl.ResolveReference(pathUrl)
	if hb.Debug {
		hb.Log.Infof("Fetching JSON from: %s", jsonUrl)
	}
	request, err := http.NewRequest("GET", jsonUrl.String(), nil)
	if err != nil {
		return jsonUrl, err
	}
	request.Header.Add("hue-application-key", applicationKey)
	client := hb.getClient()
	response, err := client.Do(request)
	if err != nil {
		return jsonUrl, err
	}
	defer response.Body.Close()
	return jsonUrl, json.NewDecoder(response.Body).Decode(v)
}

func (hb *HueBridge) getClient() *http.Client {
	if hb.cachedClient == nil {
		transport := &http.Transport{
			ResponseHeaderTimeout: time.Duration(hb.Timeout) * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		}
		hb.cachedClient = &http.Client{
			Transport: transport,
			Timeout:   time.Duration(hb.Timeout) * time.Second,
		}
	}
	return hb.cachedClient
}

func init() {
	inputs.Add("huebridge", func() telegraf.Input {
		return NewHueBridge()
	})
}
