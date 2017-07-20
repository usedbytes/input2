package ds4

import (
	"fmt"
	"log"
	"github.com/gvalkov/golang-evdev"
	"github.com/jochenvg/go-udev"
	"github.com/usedbytes/input2"
)

type subscriber struct {
	id int
	stop <-chan bool
	die chan bool
	events chan evdev.InputEvent
}

const driverName = "DualShock 4"
type Driver struct { }

func (d Driver) Name() string {
	return driverName
}

func (d Driver) MatchDevice(a *udev.Device) bool {
	return MatchDevice(a, nil)
}

func (d Driver) CompareDevices(b *udev.Device, a *udev.Device) bool {
	return MatchDevice(a, b)
}

func (d Driver) Bind(syspath string) input2.Source {
	return NewGamepad(syspath)
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
		select {
		case <-s.stop:
		case <-s.die:
			g.stopChan <-s.id
		}
	}(s)
}

func (g *Gamepad) removeSubscriber(id int) {
	log.Printf("Subscriber %d is done\n", id)

	s := g.subs[id]
	if s == nil {
		return
	}

	delete(g.subs, id)
	close(s.die)
	close(s.events)
}

func (g *Gamepad) checkDeviceRemoved(d *udev.Device) bool {
	log.Printf("Monitor event. %v\n", d)

	if d.Action() != "remove" {
		log.Printf("Not remove %v\n", d)
		return false
	}

	if !MatchDevice(g.device, d) {
		log.Printf("Not matching %v\n", d)
		return false
	}

	log.Printf("Device %s went away\n", g.sysdir)
	return true
}

func (g *Gamepad) stop() {
	g.monitorStop <-struct {}{}
	for _ = range g.monitorChan { }

	// Kill off all the subscriber stop threads
	go func() {
		for _, s := range g.subs {
			s.die <-true
		}
	}()

	// Remove each subscriber as its stop thread dies
	for {
		id := <-g.stopChan
		g.removeSubscriber(id)

		if len(g.subs) == 0 {
			break
		}
	}
	close(g.stopChan)

	// FIXME: There's a race on this if someone subscribes at the same time
	// Drop any subscribers we didn't actually add yet
	select {
	case s := <-g.subChan:
		close(s.die)
		close(s.events)
	default:
		break
	}
	close(g.subChan)
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
			if g.checkDeviceRemoved(d) {
				g.stop()
				return
			}
		}
	}
}

func MatchDevice(a *udev.Device, b *udev.Device) bool {
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
	if g.device == nil || !MatchDevice(g.device, nil) {
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

func (g *Gamepad) Subscribe(stop <-chan bool) <-chan evdev.InputEvent {
	s := subscriber{
		id: g.subid,
		stop: stop,
		die: make(chan bool),
		// TODO: A subscriber must never be allowed to block the main
		// event thread
		events: make(chan evdev.InputEvent, 10),
	}

	g.subChan <- &s
	g.subid++

	return s.events
}
