// huebridge_test.go
//
// Copyright (C) 2022 Holger de Carne
//
// This software may be modified and distributed under the terms
// of the MIT license.  See the LICENSE file for details.

package huebridge

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	plugin := NewHueBridge()
	require.NotNil(t, plugin)
}

func TestSampleConfig(t *testing.T) {
	plugin := NewHueBridge()
	sampleConfig := plugin.SampleConfig()
	require.NotNil(t, sampleConfig)
}

func TestDescription(t *testing.T) {
	plugin := NewHueBridge()
	description := plugin.Description()
	require.NotNil(t, description)
}

func TestGather1(t *testing.T) {
	testServerHandler := &testServerHandler{Debug: true}
	testServer := httptest.NewServer(testServerHandler)
	defer testServer.Close()
	plugin := NewHueBridge()
	plugin.Bridges = [][]string{{testServer.URL, "applicationkey"}}
	plugin.Log = createDummyLogger()
	plugin.Debug = testServerHandler.Debug

	var a testutil.Accumulator

	require.NoError(t, a.GatherError(plugin.Gather))
	require.True(t, a.HasMeasurement("huebridge_light"))
	require.True(t, a.HasMeasurement("huebridge_temperature"))
	require.True(t, a.HasMeasurement("huebridge_light_level"))
	require.True(t, a.HasMeasurement("huebridge_motion"))
	require.True(t, a.HasMeasurement("huebridge_device_power"))
}

func TestGather2(t *testing.T) {
	testServerHandler := &testServerHandler{Debug: true}
	testServer := httptest.NewServer(testServerHandler)
	defer testServer.Close()
	plugin := NewHueBridge()
	plugin.Bridges = [][]string{{testServer.URL, "invalid_applicationkey"}}
	plugin.Log = createDummyLogger()
	plugin.Debug = testServerHandler.Debug

	var a testutil.Accumulator

	require.Error(t, a.GatherError(plugin.Gather))
}

func createDummyLogger() *dummyLogger {
	log.SetOutput(os.Stderr)
	return &dummyLogger{}
}

type dummyLogger struct{}

func (l *dummyLogger) Errorf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (l *dummyLogger) Error(args ...interface{}) {
	log.Print(args...)
}

func (l *dummyLogger) Debugf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (l *dummyLogger) Debug(args ...interface{}) {
	log.Print(args...)
}

func (l *dummyLogger) Warnf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (l *dummyLogger) Warn(args ...interface{}) {
	log.Print(args...)
}

func (l *dummyLogger) Infof(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (l *dummyLogger) Info(args ...interface{}) {
	log.Print(args...)
}

type testServerHandler struct {
	Debug bool
}

func (tsh *testServerHandler) ServeHTTP(out http.ResponseWriter, request *http.Request) {
	requestURL := request.URL.String()
	if tsh.Debug {
		log.Printf("test: request URL: %s", requestURL)
	}
	if request.Header.Get("hue-application-key") != "applicationkey" {
		out.WriteHeader(http.StatusUnauthorized)
	} else if requestURL == "/clip/v2/resource/light" {
		tsh.serveResourceLight(out, request)
	} else if requestURL == "/clip/v2/resource/temperature" {
		tsh.serveResourceTemperature(out, request)
	} else if requestURL == "/clip/v2/resource/light_level" {
		tsh.serveResourceLightLevel(out, request)
	} else if requestURL == "/clip/v2/resource/motion" {
		tsh.serveResourceMotion(out, request)
	} else if requestURL == "/clip/v2/resource/device_power" {
		tsh.serveResourceDevicePower(out, request)
	} else if requestURL == "/clip/v2/resource/device" {
		tsh.serveResourceDevice(out, request)
	} else if requestURL == "/clip/v2/resource/room" {
		tsh.serveResourceRoom(out, request)
	}
}

const testResourceLight = `
{
	"errors":[
	  
	],
	"data":[
	  {
		"id":"519df633-bcad-489e-a490-353b6bfaf2bf",
		"id_v1":"/lights/4",
		"metadata":{
		  "archetype":"candle_bulb",
		  "name":"Lamp 1"
		},
		"mode":"normal",
		"on":{
		  "on":false
		},
		"owner":{
		  "rid":"86b46c71-ba47-4deb-99e2-6e5ae5815a8d",
		  "rtype":"device"
		},
		"type":"light"
	  },
	  {
		"id":"ad90a702-a167-4a7c-8bf0-c87cc5938855",
		"id_v1":"/lights/8",
		"metadata":{
		  "archetype":"sultan_bulb",
		  "name":"Lamp 2"
		},
		"mode":"normal",
		"on":{
		  "on":true
		},
		"owner":{
		  "rid":"5c8131ae-c187-4408-98d3-c362c5f777a3",
		  "rtype":"device"
		},
		"type":"light"
	  },
	  {
		"id":"be27e7a9-acd4-42da-a1e1-ceac9e279681",
		"id_v1":"/lights/2",
		"metadata":{
		  "archetype":"classic_bulb",
		  "name":"Lamp 3"
		},
		"mode":"normal",
		"on":{
		  "on":true
		},
		"owner":{
		  "rid":"b9e3eca4-c718-4445-9833-aa5f349c8219",
		  "rtype":"device"
		},
		"type":"light"
	  },
	  {
		"id":"22e46126-e936-4784-a628-7e8f084e9019",
		"id_v1":"/lights/9",
		"metadata":{
		  "archetype":"sultan_bulb",
		  "name":"Lamp 4"
		},
		"mode":"normal",
		"on":{
		  "on":true
		},
		"owner":{
		  "rid":"549995d5-a934-4469-ba27-6d1391633ff8",
		  "rtype":"device"
		},
		"type":"light"
	  },
	  {
		"id":"a3c5f066-f598-41eb-bae2-33d3aa34000c",
		"id_v1":"/lights/7",
		"metadata":{
		  "archetype":"candle_bulb",
		  "name":"Lamp 5"
		},
		"mode":"normal",
		"on":{
		  "on":false
		},
		"owner":{
		  "rid":"4e16129d-464c-48fc-9193-6264e463e3df",
		  "rtype":"device"
		},
		"type":"light"
	  }
	]
  }
`

func (tsh *testServerHandler) serveResourceLight(out http.ResponseWriter, request *http.Request) {
	tsh.writeJSON(out, testResourceLight)
}

const testResourceTemperature = `
{
	"errors":[
	  
	],
	"data":[
	  {
		"enabled":true,
		"id":"5606a921-4315-4d66-867a-552718df8cae",
		"id_v1":"/sensors/3",
		"owner":{
		  "rid":"92cd53c4-abff-437c-bb21-1733e74c5df5",
		  "rtype":"device"
		},
		"temperature":{
		  "temperature":20.45,
		  "temperature_valid":true
		},
		"type":"temperature"
	  }
	]
  }
`

func (tsh *testServerHandler) serveResourceTemperature(out http.ResponseWriter, request *http.Request) {
	tsh.writeJSON(out, testResourceTemperature)
}

const testResourceLightLevel = `
{
	"errors":[
	  
	],
	"data":[
	  {
		"enabled":true,
		"id":"b612b85c-1231-4aed-ac03-98250efdbb6c",
		"id_v1":"/sensors/5",
		"light":{
		  "light_level":1563,
		  "light_level_valid":true
		},
		"owner":{
		  "rid":"92cd53c4-abff-437c-bb21-1733e74c5df5",
		  "rtype":"device"
		},
		"type":"light_level"
	  }
	]
  }
`

func (tsh *testServerHandler) serveResourceLightLevel(out http.ResponseWriter, request *http.Request) {
	tsh.writeJSON(out, testResourceLightLevel)
}

const testResourceMotion = `
{
	"errors":[
	  
	],
	"data":[
	  {
		"enabled":true,
		"id":"4a50cccd-b1d7-447e-bd94-3e73b2e6097a",
		"id_v1":"/sensors/4",
		"motion":{
		  "motion":false,
		  "motion_valid":true
		},
		"owner":{
		  "rid":"92cd53c4-abff-437c-bb21-1733e74c5df5",
		  "rtype":"device"
		},
		"type":"motion"
	  }
	]
  }
`

func (tsh *testServerHandler) serveResourceMotion(out http.ResponseWriter, request *http.Request) {
	tsh.writeJSON(out, testResourceMotion)
}

const testResourceDevicePower = `
{
	"errors":[
	  
	],
	"data":[
	  {
		"id":"52d23eb6-c9b5-4641-b873-3d441888c34b",
		"id_v1":"/sensors/4",
		"owner":{
		  "rid":"92cd53c4-abff-437c-bb21-1733e74c5df5",
		  "rtype":"device"
		},
		"power_state":{
		  "battery_level":100,
		  "battery_state":"normal"
		},
		"type":"device_power"
	  }
	]
  }
`

func (tsh *testServerHandler) serveResourceDevicePower(out http.ResponseWriter, request *http.Request) {
	tsh.writeJSON(out, testResourceDevicePower)
}

const testResourceDevice = `
{
	"errors":[
	  
	],
	"data":[
	  {
		"id":"da699f9a-1098-484f-99af-8de24cb9b44f",
		"id_v1":"/lights/3",
		"metadata":{
		  "archetype":"sultan_bulb",
		  "name":"Lamp 1"
		},
		"type":"device"
	  },
	  {
		"id":"86b46c71-ba47-4deb-99e2-6e5ae5815a8d",
		"id_v1":"/lights/4",
		"metadata":{
		  "archetype":"candle_bulb",
		  "name":"Lamp 2"
		},
		"type":"device"
	  },
	  {
		"id":"9633d4eb-d7c7-4178-a4e0-98098a47419d",
		"id_v1":"/lights/12",
		"metadata":{
		  "archetype":"sultan_bulb",
		  "name":"Lamp 3"
		},
		"type":"device"
	  },
	  {
		"id":"5c8131ae-c187-4408-98d3-c362c5f777a3",
		"id_v1":"/lights/8",
		"metadata":{
		  "archetype":"sultan_bulb",
		  "name":"Lamp 4"
		},
		"type":"device"
	  },
	  {
		"id":"a58ff365-eba2-4411-8678-d5ac2c68512b",
		"id_v1":"/lights/6",
		"metadata":{
		  "archetype":"candle_bulb",
		  "name":"Lamp 5"
		},
		"type":"device"
	  },
	  {
		"id":"b9e3eca4-c718-4445-9833-aa5f349c8219",
		"id_v1":"/lights/2",
		"metadata":{
		  "archetype":"classic_bulb",
		  "name":"Lamp 6"
		},
		"type":"device"
	  },
	  {
		"id":"2d64a5dc-c19f-47c9-bb68-1e6391a17a69",
		"id_v1":"/lights/5",
		"metadata":{
		  "archetype":"candle_bulb",
		  "name":"Lamp 7"
		},
		"type":"device"
	  },
	  {
		"id":"b090d566-2fd5-4fac-b12c-b23e4d82d349",
		"id_v1":"",
		"metadata":{
		  "archetype":"bridge_v2",
		  "name":"huebridge1"
		},
		"type":"device"
	  },
	  {
		"id":"549995d5-a934-4469-ba27-6d1391633ff8",
		"id_v1":"/lights/9",
		"metadata":{
		  "archetype":"sultan_bulb",
		  "name":"Lamp 8"
		},
		"type":"device"
	  },
	  {
		"id":"5527b2a3-c1b5-4eca-b2e3-94ed5ea9d292",
		"id_v1":"/lights/11",
		"metadata":{
		  "archetype":"sultan_bulb",
		  "name":"Lamp 9"
		},
		"type":"device"
	  },
	  {
		"id":"4e16129d-464c-48fc-9193-6264e463e3df",
		"id_v1":"/lights/7",
		"metadata":{
		  "archetype":"candle_bulb",
		  "name":"Lamp 10"
		},
		"type":"device"
	  },
	  {
		"id":"92cd53c4-abff-437c-bb21-1733e74c5df5",
		"id_v1":"/sensors/4",
		"metadata":{
		  "archetype":"unknown_archetype",
		  "name":"Motion sensor"
		},
		"type":"device"
	  }
	]
  }
  `

func (tsh *testServerHandler) serveResourceDevice(out http.ResponseWriter, request *http.Request) {
	tsh.writeJSON(out, testResourceDevice)
}

const testResourceRoom = `
{
	"errors":[
	  
	],
	"data":[
	  {
		"children":[
		  {
			"rid":"5527b2a3-c1b5-4eca-b2e3-94ed5ea9d292",
			"rtype":"device"
		  }
		],
		"id":"3e13b206-df64-40c5-ad4a-61b2b7167ce5",
		"id_v1":"/groups/5",
		"metadata":{
		  "archetype":"bedroom",
		  "name":"Bett"
		},
		"type":"room"
	  },
	  {
		"children":[
		  {
			"rid":"da699f9a-1098-484f-99af-8de24cb9b44f",
			"rtype":"device"
		  }
		],
		"id":"e998cc46-e091-41c2-bdb9-df7ac849982e",
		"id_v1":"/groups/1",
		"metadata":{
		  "archetype":"living_room",
		  "name":"Wohnzimmer"
		},
		"type":"room"
	  },
	  {
		"children":[
		  {
			"rid":"9633d4eb-d7c7-4178-a4e0-98098a47419d",
			"rtype":"device"
		  }
		],
		"id":"e1b696b9-21a4-4a2e-8f66-6e5463581069",
		"id_v1":"/groups/2",
		"metadata":{
		  "archetype":"bedroom",
		  "name":"Schlafzimmer"
		},
		"type":"room"
	  },
	  {
		"children":[
		  {
			"rid":"5c8131ae-c187-4408-98d3-c362c5f777a3",
			"rtype":"device"
		  },
		  {
			"rid":"b9e3eca4-c718-4445-9833-aa5f349c8219",
			"rtype":"device"
		  },
		  {
			"rid":"549995d5-a934-4469-ba27-6d1391633ff8",
			"rtype":"device"
		  }
		],
		"id":"e8006e01-92a3-4bc7-9102-a768259187b0",
		"id_v1":"/groups/3",
		"metadata":{
		  "archetype":"other",
		  "name":"Flur"
		},
		"type":"room"
	  },
	  {
		"children":[
		  {
			"rid":"86b46c71-ba47-4deb-99e2-6e5ae5815a8d",
			"rtype":"device"
		  },
		  {
			"rid":"a58ff365-eba2-4411-8678-d5ac2c68512b",
			"rtype":"device"
		  },
		  {
			"rid":"2d64a5dc-c19f-47c9-bb68-1e6391a17a69",
			"rtype":"device"
		  },
		  {
			"rid":"4e16129d-464c-48fc-9193-6264e463e3df",
			"rtype":"device"
		  }
		],
		"id":"77db55a9-276f-45cc-91d8-0ab4966b288a",
		"id_v1":"/groups/4",
		"metadata":{
		  "archetype":"recreation",
		  "name":"Arbeitszimmer"
		},
		"type":"room"
	  }
	]
  }
`

func (tsh *testServerHandler) serveResourceRoom(out http.ResponseWriter, request *http.Request) {
	tsh.writeJSON(out, testResourceRoom)
}

func (tsh *testServerHandler) writeJSON(out http.ResponseWriter, json string) {
	out.Header().Add("Content-Type", "application/json")
	_, _ = out.Write([]byte(json))
}
