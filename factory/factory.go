package factory

import (
	"log"

	"github.com/jochenvg/go-udev"
	"github.com/usedbytes/input2/gamepad/ds4"
	"github.com/usedbytes/input2"
)

var drivers = [...]input2.Driver{ ds4.Driver{}, }

func bindDevice(d *udev.Device) input2.Source {
	for _, drv := range drivers {
		if drv.MatchDevice(d) {
			log.Printf("Attempting to bind driver '%s' to device '%p'\n",
				   drv.Name(), d)
			return drv.Bind(d.Syspath())
		}
	}

	return nil
}

func monitorDevices(u *udev.Udev, deviceChan <-chan *udev.Device, sourceChan chan<- input2.Source) {
	for {
		select {
		case d := <-deviceChan:
			switch d.Action() {
			case "add", "":
				s := bindDevice(d)
				if s != nil {
					sourceChan <-s
				}
			}

		}
	}
}


func Monitor() chan input2.Source {
	u := &udev.Udev{}

	done := make(chan struct{})
	deviceChan := make(chan *udev.Device, 10)
	sourceChan := make(chan input2.Source, 10)

	go monitorDevices(u, deviceChan, sourceChan)

	m := u.NewMonitorFromNetlink("udev")
	monitorChan, err := m.DeviceChan(done)
	if err != nil {
		return nil
	}
	go func() {
		for {
			d := <-monitorChan
			deviceChan <- d
		}
	}()

	e := u.NewEnumerate()
	e.AddMatchIsInitialized()
	devices, _ := e.Devices()
	for i := range devices {
		d := devices[i]
		deviceChan <- d
	}

	return sourceChan
}
