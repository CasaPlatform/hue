// Copyright © 2016 Casa Platform
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hue

import (
	"errors"
	"strconv"
	"strings"

	"github.com/casaplatform/casa"
	"github.com/inhies/GoHue"
)

// Endpoints are designed to be self documenting, hence the Params and Description
// fields. A pointer to their parent Light is included so they can call other
// endpoints, or call the parent Bridge's MessageBus client.
type endpoint struct {
	Params      string
	Description string
	SetState    func(light *Light, data string) error
	GetState    func(light *Light, topic string) (string, error)

	light *Light
}

// Sets the light endpoint to the specified state, returns an error if it
// doesn't exist
func (l *Light) setEndpointState(endpoint, payload string) error {
	point := l.endpoints[endpoint]
	if point == nil {
		return errors.New("Unknown endpoint: " + endpoint)
	}
	return point.SetState(l, payload)
}

// A list of all endpoints applicable to a hue.Light. Some might be missing.
// Implemented just to get the package built and working.
var endpoints = map[string]*endpoint{
	"On": {
		Params: "on bool", Description: "Turns the light on or off",
		SetState: func(l *Light, payload string) error {
			on, err := strconv.ParseBool(payload)
			if err != nil {
				return err
			}
			if on {
				err = l.Light.On()
			} else {
				err = l.Light.Off()
			}

			if err != nil {
				return err
			}

			return l.bridge.client.PublishMessage(casa.Message{
				Topic:   l.Path + "/On",
				Payload: []byte(strconv.FormatBool(on)),
				Retain:  true,
			})

		},
		GetState: func(light *Light, topic string) (string, error) {
			return strconv.FormatBool(light.Light.State.On), nil
		}},

	"Brightness": {
		Params:      "percent int",
		Description: "Sets the light brightness to `percent` percent",
		SetState: func(l *Light, payload string) error {
			value, err := strconv.Atoi(payload)
			if err != nil {
				return err
			}

			err = l.Light.SetBrightness(value)
			if err != nil {
				return err
			}

			return l.bridge.client.PublishMessage(casa.Message{
				Topic:   l.Path + "/Brightness",
				Payload: []byte(payload),
				Retain:  true,
			})
		},
		GetState: func(light *Light, topic string) (string, error) {
			return strconv.FormatUint(uint64(light.Light.State.Bri), 10), nil
		}},

	"Hue": {
		Params:      "value uint16",
		Description: "Sets the hue to the specified value from 1-65535",
		SetState: func(l *Light, payload string) error {
			if h, err := strconv.ParseUint(payload, 10, 16); err == nil {
				state := hue.LightState{
					Hue: uint16(h),
					On:  true,
				}

				err = l.Light.SetState(state)
				if err != nil {
					return err
				}
				return l.bridge.client.PublishMessage(casa.Message{
					Topic:   l.Path + "/Hue",
					Payload: []byte(payload),
					Retain:  true,
				})

			}

			return errors.New("Invalid payload " + payload)
		},
		GetState: func(light *Light, topic string) (string, error) {
			return strconv.FormatUint(uint64(light.Light.State.Hue), 10), nil
		}},

	"Saturation": {
		Params:      "value uint",
		Description: "Sets the saturation to the specified value from 0-254",
		SetState: func(l *Light, payload string) error {
			if h, err := strconv.ParseUint(payload, 10, 8); err == nil {
				state := hue.LightState{
					Sat: uint8(h),
					On:  true,
				}

				err = l.Light.SetState(state)
				if err != nil {
					return err
				}
				return l.bridge.client.PublishMessage(casa.Message{
					Topic:   l.Path + "/Saturation",
					Payload: []byte(payload),
					Retain:  true,
				})

			}

			return errors.New("Invalid payload " + payload)
		},
		GetState: func(light *Light, topic string) (string, error) {
			return strconv.FormatUint(uint64(light.Light.State.Saturation), 10), nil
		}},

	"Effect": {
		Params:      "effect string",
		Description: "Sets the effect mode. Acceptable values are 'Colorloop' or 'None'",
		SetState: func(l *Light, payload string) error {
			state := new(hue.LightState)
			state.Effect = payload
			state.On = true

			err := l.Light.SetState(*state)
			if err != nil {
				return err
			}

			return l.bridge.client.PublishMessage(casa.Message{
				Topic:   l.Path + "/Effect",
				Payload: []byte(payload),
				Retain:  true,
			})

		},
		GetState: func(light *Light, topic string) (string, error) {
			return light.Light.State.Effect, nil
		}},

	"XY Color": {
		Params:      "x,y float",
		Description: "Sets the light to the  `x,y` positions on the HSL color spectrum",
		SetState: func(l *Light, payload string) error {
			colors := strings.Split(payload, ",")
			if len(colors) != 2 {
				return errors.New("invalid colors")
			}

			x, err := strconv.ParseFloat(colors[0], 32)
			if err != nil {
				return err
			}

			y, err := strconv.ParseFloat(colors[1], 32)
			if err != nil {
				return err
			}

			err = l.Light.SetColor(&[2]float32{float32(x), float32(y)})
			if err != nil {
				return err
			}

			return l.bridge.client.PublishMessage(casa.Message{

				Topic:   l.Path + "/XY Color",
				Payload: []byte(payload),
				Retain:  true,
			})
			if err != nil {
				return err
			}

			return l.bridge.client.PublishMessage(casa.Message{

				Topic:   l.Path + "/Color Name",
				Payload: []byte("None"),
				Retain:  true,
			})

		},
		GetState: func(light *Light, topic string) (string, error) {
			return strconv.FormatFloat(float64(light.Light.State.XY[0]), 'f', -1, 32) +
				"," + strconv.FormatFloat(float64(light.Light.State.XY[1]), 'f', -1, 32), nil
		}},

	"Color Name": {
		Params:      "name string",
		Description: "Sets the light to the predefined color",
		SetState: func(l *Light, payload string) error {
			// Check to ensure the named color exists in our map
			if payload == "None" || payload == "" {
				return l.bridge.client.PublishMessage(casa.Message{

					Topic:   l.Path + "/Color Name",
					Payload: []byte("None"),
					Retain:  true,
				})
			}

			if Colors[payload] == nil {
				return errors.New("Invalid color name")
			}

			// Set the light to the color
			err := l.Light.SetColor(Colors[payload])
			if err != nil {
				return err
			}

			// Update the MQTT topic for the light color
			err = l.bridge.client.PublishMessage(casa.Message{

				Topic:   l.Path + "/Color Name",
				Payload: []byte(payload),
				Retain:  true,
			})
			if err != nil {
				return err
			}

			// Update the XY Color topic with these colors

			return l.bridge.client.PublishMessage(casa.Message{

				Topic:   l.Path + "/XY Color",
				Payload: []byte(strconv.FormatFloat(float64(Colors[payload][0]), 'f', -1, 32) + "," + strconv.FormatFloat(float64(Colors[payload][1]), 'f', -1, 32)),
				Retain:  true,
			})
		},
		GetState: func(light *Light, topic string) (string, error) {
			return "", nil
		}},

	"Color Temp": {
		Params:      "value int",
		Description: "Sets the mired color temperature to the specified value",
		SetState: func(l *Light, payload string) error {
			if h, err := strconv.ParseUint(payload, 10, 16); err == nil {
				state := new(hue.LightState)
				state.CT = uint16(h)
				state.On = true

				err := l.Light.SetState(*state)
				if err != nil {
					return err
				}
				return l.bridge.client.PublishMessage(casa.Message{

					Topic:   l.Path + "/Color Temp",
					Payload: []byte(payload),
					Retain:  true,
				})

			}

			return errors.New("Invalid payload " + payload)
		},
		GetState: func(light *Light, topic string) (string, error) {
			return strconv.FormatUint(uint64(light.Light.State.Saturation), 8), nil
		}},

	"Alert": {
		Params:      "selected string",
		Description: "Sets the light alert state. Valid values are 'Selected' or 'None'",
		SetState: func(l *Light, payload string) error {
			state := hue.LightState{
				Alert: payload,
				On:    true,
			}

			err := l.bridge.client.PublishMessage(casa.Message{

				Topic:   l.Path + "/Alert",
				Payload: []byte(payload),
				Retain:  true,
			})
			if err != nil {
				return err
			}
			return l.Light.SetState(state)

		},
		GetState: func(light *Light, topic string) (string, error) {
			return light.Light.State.Alert, nil
		}},

	"Color Mode": {
		Params:      "read only",
		Description: "Specifies the last mode used for choosing colors. Values are 'hs' for Hue and Saturation, 'xy' for XY and 'ct' for Color Temperature.",

		GetState: func(l *Light, payload string) (string, error) {
			return l.Light.State.ColorMode, nil
		}},
}
