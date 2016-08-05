package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	"github.com/brutella/log"

	"github.com/ablyler/nest"
)

type HKThermostat struct {
	accessory *accessory.Accessory
	transport hc.Transport

	thermostat *accessory.Thermostat
}

var (
	thermostats   map[string]*HKThermostat
	nestPin       string
	homekitPin    string
	productID     string
	productSecret string
	state         string
)

func logEvent(device *nest.Thermostat) {
	data, _ := json.MarshalIndent(device, "", "  ")
	fmt.Println(string(data))
}

func Connect() {
	client := nest.New(productID, state, productSecret, nestPin)
	client.Authorize()
	// fmt.Println(client.Token)

	client.DevicesStream(func(devices *nest.Devices, err error) {
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		for _, device := range devices.Thermostats {
			// logEvent(device)

			hkThermostat := GetHKThermostat(device)
			hkThermostat.thermostat.Thermostat.CurrentTemperature.SetValue(float64(device.AmbientTemperatureC))
			hkThermostat.thermostat.Thermostat.TargetTemperature.SetValue(float64(device.TargetTemperatureC))

			var mode int
			switch device.HvacMode {
			case "heat":
				mode = characteristic.TargetHeatingCoolingStateHeat
			case "cool":
				mode = characteristic.TargetHeatingCoolingStateCool
			case "off":
				mode = characteristic.TargetHeatingCoolingStateOff
			default:
				mode = characteristic.TargetHeatingCoolingStateAuto
			}

			hkThermostat.thermostat.Thermostat.TargetHeatingCoolingState.SetValue(mode)

			switch device.HvacState {
			case "heating":
				mode = characteristic.CurrentHeatingCoolingStateHeat
			case "cooling":
				mode = characteristic.CurrentHeatingCoolingStateHeat
			default:
				mode = characteristic.CurrentHeatingCoolingStateOff
			}

			hkThermostat.thermostat.Thermostat.CurrentHeatingCoolingState.SetValue(mode)
		}
	})
}

// GetHKThermostat reaches out to the nest device
func GetHKThermostat(nestThermostat *nest.Thermostat) *HKThermostat {
	hkThermostat, found := thermostats[nestThermostat.DeviceID]
	if found {
		return hkThermostat
	}

	log.Printf("[INFO] Creating New HKThermostat for %s", nestThermostat.Name)

	info := accessory.Info{
		Name:         nestThermostat.Name,
		Manufacturer: "Nest",
	}

	thermostat := accessory.NewThermostat(info, float64(nestThermostat.AmbientTemperatureC), 9, 32, float64(0.5))

	config := hc.Config{Pin: homekitPin}
	transport, err := hc.NewIPTransport(config, thermostat.Accessory)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		transport.Start()
	}()

	hkThermostat = &HKThermostat{
		accessory:  thermostat.Accessory,
		transport:  transport,
		thermostat: thermostat,
	}
	thermostats[nestThermostat.DeviceID] = hkThermostat

	thermostat.Thermostat.TargetTemperature.OnValueRemoteUpdate(func(target float64) {
		log.Printf("[INFO] Changed Target Temp for %s", nestThermostat.Name)
		nestThermostat.SetTargetTempC(float32(target))
	})

	thermostat.Thermostat.TargetHeatingCoolingState.OnValueRemoteUpdate(func(mode int) {
		log.Printf("[INFO] Changed Mode for %s", nestThermostat.Name)

		switch mode {
		case characteristic.TargetHeatingCoolingStateOff:
			nestThermostat.SetHvacMode(nest.Off)
		case characteristic.TargetHeatingCoolingStateHeat:
			nestThermostat.SetHvacMode(nest.Heat)
		case characteristic.TargetHeatingCoolingStateCool:
			nestThermostat.SetHvacMode(nest.Cool)
		default:
			nestThermostat.SetHvacMode(nest.HeatCool)
		}

	})

	return hkThermostat
}

func main() {
	thermostats = map[string]*HKThermostat{}

	productIDArg := flag.String("product-id", "", "Nest provided product id")
	productSecretArg := flag.String("product-secret", "", "Nest provided product secret")
	stateArg := flag.String("state", "", "A value you create, used during OAuth")
	nestPinArg := flag.String("nest-pin", "", "PIN generated from the Nest site")
	homekitPinArg := flag.String("homekit-pin", "", "PIN you create to be used to pair Nest with HomeKit")
	verboseArg := flag.Bool("v", false, "Whether or not log output is displayed")

	flag.Parse()

	productID = *productIDArg
	productSecret = *productSecretArg
	state = *stateArg
	nestPin = *nestPinArg
	homekitPin = *homekitPinArg

	if !*verboseArg {
		log.Info = false
		log.Verbose = false
	}

	hc.OnTermination(func() {
		os.Exit(1)
	})

	Connect()
}
