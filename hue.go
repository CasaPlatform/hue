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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/casaplatform/casa"
	"github.com/casaplatform/casa/cmd/casa/environment"
	"github.com/casaplatform/mqtt"
	"github.com/inhies/GoHue"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	Namespace = "Hue"
	SvcPrefix = "Service"
	DevPrefix = "Device"
)

// TODO: Add more colors from http://www.developers.meethue.com/documentation/hue-xy-values
// TODO: Make colors work with multiple color gamuts (A,B,C)
// TODO: Make getting color names from values possible

// Simple pre-defined colors
var Colors = map[string]*[2]float32{
	"Red":    hue.RED,
	"Yellow": hue.YELLOW,
	"Orange": hue.ORANGE,
	"Green":  hue.GREEN,
	"Cyan":   hue.CYAN,
	"Blue":   hue.BLUE,
	"Purple": hue.PURPLE,
	"Pink":   hue.PINK,
	"White":  hue.WHITE,
}

type Bridge struct {
	IP     string
	User   string
	client casa.MessageClient

	m      sync.RWMutex
	lights map[string]*Light

	bridge *hue.Bridge
	casa.Logger
}

type Light struct {
	Light *hue.Light
	Path  string

	m         sync.RWMutex
	endpoints map[string]*endpoint

	bridge *Bridge
}

func init() {
	environment.RegisterService("hue", &Bridge{})
}

func NewBridge(ip string) *Bridge {
	return &Bridge{
		IP: ip,
	}
}
func (b *Bridge) UseLogger(logger casa.Logger) {
	b.Logger = logger
}

// Handle aabode messages
func (b *Bridge) handler(msg *casa.Message, err error) {
	switch {
	case err != nil:
		b.Log(err)
		return
	case msg != nil:
		m := strings.Split(msg.Topic, "/")

		if m[len(m)-1] == "Register" {
			newbridge, err := hue.NewBridge(m[len(m)-2])
			if err != nil {
				b.Log("Unable to connect to Hue bridge:", err)
				return
			}
			b.Log("Press the link button on the Hue bridge")
			var token string
			for i := 0; i < 12; i++ {
				time.Sleep(5 * time.Second)
				token, err = newbridge.CreateUser("Casa" + strconv.FormatInt(time.Now().Unix(), 10))
				if err != nil {
					b.Log(err)
				}
			}
			if token == "" {
				b.Log("Unable to create user on Hue bridge. Please try again")
				return
			}
			b.Log("Token created:", token)

		}

		// We only care about commands sent to us
		if m[len(m)-1] != "Set" {
			return
		}

		b.m.RLock()
		light := b.lights[m[len(m)-3]]
		defer b.m.RUnlock()

		if light == nil {
			b.Log(errors.New("Invalid Hue device specified: " + m[len(m)-3]))
			return
		}

		endpoint := m[len(m)-2]

		err = light.setEndpointState(endpoint, string(msg.Payload))
		if err != nil {
			b.Log(err)
		}
		return
	default:
		b.Log(errors.New("Handler called with nil message and error"))
	}

}
func (b *Bridge) Start(config *viper.Viper) error {
	if config.IsSet("BridgeIP") &&
		config.IsSet("User") {
		b.IP = config.GetString("BridgeIP")
		b.User = config.GetString("User")
	} else {
		// Need to setup a new hue bridge here
		return errors.New("No valid Hue bridge found in config")
	}

	client, err := mqtt.NewClient(
		"tcp://127.0.0.1:1883",
		mqtt.Timeout(5*time.Second),
	)

	if err != nil {
		return err
	}

	bridge, err := hue.NewBridge(b.IP)
	if err != nil {
		return err
	}

	err = bridge.Login(b.User)
	if err != nil {
		return err
	}

	lights, err := bridge.GetAllLights()
	if err != nil {
		return err
	}

	b.client = client
	b.lights = make(map[string]*Light)

	for i := 0; i < len(lights); i++ {
		l := lights[i]
		id := Namespace + "/" + bridge.Info.Device.FriendlyName + "/Light/" + l.Name
		light := &Light{
			Light:     &l,
			Path:      id,
			endpoints: endpoints,

			bridge: b,
		}
		b.lights[l.Name] = light

		for point, data := range endpoints {
			data.light = light
			err := b.client.PublishMessage(casa.Message{
				Topic:   "New/" + id + "/" + point,
				Payload: []byte(data.Params + " : " + data.Description),
				Retain:  true,
			})

			if err != nil {
				return err
			}

			payload, err := data.GetState(light, id+"/"+point)
			if err != nil {
				return err
			}

			err = b.client.PublishMessage(casa.Message{
				Topic:   id + "/" + point,
				Payload: []byte(payload),
				Retain:  true,
			})
			if err != nil {
				return err
			}
		}

		err := b.client.Subscribe(id + "/#")
		if err != nil {
			return err
		}
	}

	b.client.Handle(b.handler)
	return nil
}

func (b *Bridge) Stop() error {
	if b.client != nil {
		return b.client.Close()
	}
	return nil
}
