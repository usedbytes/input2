package ds4

import (
	"fmt"
	"log"
	"github.com/gvalkov/golang-evdev"
	"github.com/jochenvg/go-udev"
)

type subscriber struct {
	id int
	stop <-chan bool
	events chan evdev.InputEvent
}

type Gamepad struct {
	subid int
	sysdir string

	udev *udev.Udev
	device *udev.Device
	monitor *udev.Monitor
	monitorStop chan struct{}
	monitorChan <-chan *udev.Device

	subs map[int]*subscriber
	subChan chan *subscriber
	stopChan chan int
}

func (g *Gamepad) addSubscriber(s *subscriber) {
	log.Printf("New subscriber: %d\n", s.id)

	g.subs[s.id] = s
	go func(s *subscriber) {
		<-s.stop
		g.stopChan <-s.id
	}(s)
}

func (g *Gamepad) removeSubscriber(id int) {
	log.Printf("Subscriber %d is done\n", id)

	s := g.subs[id]
	if s == nil {
		return
	}

	delete(g.subs, id)
	close(s.events)
}

func (g *Gamepad) checkMonitorEvent(d *udev.Device) {
	log.Printf("Monitor event. %v\n", d)

	if d.Action() != "remove" {
		log.Printf("Not remove\n", d)
		return
	}

	if !matchDevice(g.device, d) {
		log.Printf("Not matching\n", d)
		return
	}

	log.Printf("Device %s went away\n", g.sysdir)
}

func (g *Gamepad) run() {
	log.Printf("Running...\n")
	for {
		select {
		case s := <-g.subChan:
			g.addSubscriber(s)
		case i := <-g.stopChan:
			g.removeSubscriber(i)
		case d := <-g.monitorChan:
			g.checkMonitorEvent(d)
		}
	}
}

func matchDevice(a *udev.Device, b *udev.Device) bool {
	if a.Subsystem() != "hid" {
		return false
	}

	if a.PropertyValue("DRIVER") != "sony" {
		return false
	}

	if b != nil {
		a_uniq := a.PropertyValue("HID_UNIQ")
		b_uniq := b.PropertyValue("HID_UNIQ")
		a_phys := a.PropertyValue("HID_PHYS")
		b_phys := b.PropertyValue("HID_PHYS")

		if (a_uniq != b_uniq) || (a_phys != b_phys) {
			return false
		}
	}

	return true
}

func (g *Gamepad) initUdev() error {
	var err error

	g.udev = &udev.Udev{}
	g.device = g.udev.NewDeviceFromSyspath(g.sysdir)
	if g.device == nil || !matchDevice(g.device, nil) {
		return fmt.Errorf("Couldn't get device '%s'", g.sysdir)
	}

	g.monitor = g.udev.NewMonitorFromNetlink("udev")
	if g.monitor == nil {
		return fmt.Errorf("Couldn't get udev monitor")
	}
	g.monitor.FilterAddMatchSubsystem(g.device.Subsystem())
	g.monitorStop = make(chan struct{})
	g.monitorChan, err = g.monitor.DeviceChan(g.monitorStop)
	if err != nil {
		return err
	}

	return nil
}

func NewGamepad(sysdir string) *Gamepad {
	log.Printf("Gamepad %s\n", sysdir)
	g := &Gamepad {
		subid: 0,
		sysdir: sysdir,
		subs: make(map[int]*subscriber),
		subChan: make(chan *subscriber),
		stopChan: make(chan int, 5),
	}

	err := g.initUdev()
	if err != nil {
		log.Print(err)
		return nil
	}

	go g.run()

	return g
}

func (g *Gamepad) Subscribe(stop <-chan bool) chan evdev.InputEvent {
	s := subscriber{
		id: g.subid,
		stop: stop,
		// TODO: A subscriber must never be allowed to block the main
		// event thread
		events: make(chan evdev.InputEvent, 10),
	}

	g.subChan <- &s
	g.subid++

	return s.events
}
