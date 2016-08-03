package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/brutella/log"
  "github.com/brutella/hc"
	"github.com/brutella/hc/hap"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/service"
	"github.com/brutella/hc/characteristic"

	"github.com/ablyler/nest"
)

type HKThermostat struct {
	accessory *accessory.Accessory
	transport hc.Transport
  characteristic *characteristic.Characteristic
  service *service.Thermostat
	thermostat accessory.Thermostat
}

var (
	thermostats        map[string]*HKThermostat
	nestPin            string
	homekitPin         string
	productID          string
	productSecret      string
	state              string
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

			hkThermostat := GetHKThermostat(device);
			hkThermostat.thermostat.Thermostat.CurrentTemperature.SetValue(float64(device.AmbientTemperatureC));
			hkThermostat.thermostat.Thermostat.TargetTemperature.SetValue(float64(device.TargetTemperatureC));

			//var targetMode model.HeatCoolModeType
      var targetMode characteristic.CurrentHeatingCoolingState

			switch device.HvacMode {
        //https://github.com/brutella/hc/blob/master/characteristic/target_heating_cooling_state.go
        //https://github.com/brutella/hc/blob/master/characteristic/current_heating_cooling_state.go
			case "heat":
				//targetMode = model.HeatCoolModeHeat
        // nope targetMode = 1
        targetMode = characteristic.CurrentHeatingCoolingStateHeat
    	case "cool":
				//targetMode = model.HeatCoolModeCool
        targetMode = 2
			case "off":
				//targetMode = model.HeatCoolModeOff
        targetMode = 0
			default:
				//targetMode = model.HeatCoolModeAuto
        targetMode = 3
			}

			//hkThermostat.thermostat.Thermostat.SetTargetMode(targetMode)
      hkThermostat.thermostat.Thermostat.TargetHeatingCoolingState(targetMode)

			var mode Thermostat.HeatCoolModeType

			switch device.HvacState {
			case "heating":
				//mode = service.HeatCoolModeHeat
        mode = Thermostat.TargetHeatingCoolingStateHeat
			case "cooling":
				//mode = service.HeatCoolModeCool
        mode = TargetHeatingCoolingStateCool
			default:
				//mode = service.HeatCoolModeOff
        mode = Thermostat.TargetHeatingCoolingStateOff
			}

			//hkThermostat.thermostat.SetMode(mode)
		  hkThermostat.thermostat.Thermostat.SetMode(mode)
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

	info := model.Info{
		Name:         nestThermostat.Name,
		Manufacturer: "Nest",
	}

	thermostat := accessory.NewThermostat(info, float64(nestThermostat.AmbientTemperatureC), 9, 32, float64(0.5))

	config := hap.Config{Pin: homekitPin}
	transport, err := hap.NewIPTransport(config, thermostat.Accessory)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		transport.Start()
	}()

	hkThermostat = &HKThermostat{thermostat.Accessory, transport, thermostat}
	thermostats[nestThermostat.DeviceID] = hkThermostat

	thermostat.OnTargetTempChange(func(target float64) {
		log.Printf("[INFO] Changed Target Temp for %s", nestThermostat.Name)
		nestThermostat.SetTargetTempC(float32(target))
	})

	thermostat.OnTargetModeChange(func(mode model.HeatCoolModeType) {
		log.Printf("[INFO] Changed Mode for %s", nestThermostat.Name)

		if mode == model.HeatCoolModeHeat {
			nestThermostat.SetHvacMode(nest.Heat)
		} else if mode == model.HeatCoolModeCool {
			nestThermostat.SetHvacMode(nest.Cool)
		} else if mode == model.HeatCoolModeOff {
			nestThermostat.SetHvacMode(nest.Off)
		} else {
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

	hap.OnTermination(func() {
		os.Exit(1)
	})

	Connect()
}
