// +build example
//
// Do not build by default.

/*
How to run:
Need 2 Tello drones and 2 WiFi adapters.
Connect to the drone's Wi-Fi network from your computer. It will be named something like "TELLO-XXXXXX".
Here is the trick:
Manually setup IP addresses to 192.168.10.2  for first WiFi adapter, and 192.168.10.3 for second WiFi adapter.
Once you are connected to both drones, you can run the Gobot code on your computer to control the drones.

	go run examples/tello_multiple.go
*/

package main

import (
	"time"

	"gobot.io/x/gobot"
	"gobot.io/x/gobot/platforms/dji/tello"
)

var drones []*tello.Driver

func main() {
	drones = append(drones,tello.NewDriver("192.168.10.2:8888"))
	//drones = append(drones,tello.NewDriver("192.168.10.3:8889"))
	//drones = append(drones,tello.NewDriver("192.168.10.4:8890"))

	work := func() {
		drones[0].TakeOff()
		/*
		for _, drone := range drones {
			drone.TakeOff()
			
		}
		*/

		gobot.After(5*time.Second, func() {
			drones[0].Land()
			/*
			for _, drone := range drones {
				drone.Land()
			}
			*/
		})
	}

	robot := gobot.NewRobot("tello",
		[]gobot.Connection{},
		[]gobot.Device{drones[0]},
		work,
	)

	robot.Start()
}
