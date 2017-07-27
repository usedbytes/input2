package ds4

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
	"github.com/gvalkov/golang-evdev"
	"github.com/jochenvg/go-udev"
	"github.com/usedbytes/input2"
	"github.com/usedbytes/input2/gamepad"
)

var mainDevRegexp = regexp.MustCompile("Wireless Controller$")

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
	mutex sync.Mutex
	stopped bool
	evdev *evdev.InputDevice

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
		if len(g.subs) == 0 {
			break
		}

		id := <-g.stopChan
		g.removeSubscriber(id)
	}
	close(g.stopChan)

	// Keep closing any incoming subscribers until someone closes the
	// channel
	go func() {
		for s := range g.subChan {
			close(s.die)
			close(s.events)
		}
	}()

	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.stopped = true
	close(g.subChan)
}

func (g *Gamepad) runDevice(evchan chan<- []evdev.InputEvent) {
	for {
		evs, err := g.evdev.Read()
		if err != nil {
			// TODO: We should communicate the error to subscribers
			if pe, ok := err.(*os.PathError); ok {
				log.Printf("Device Error: %s %d\n", pe.Path, pe.Err)
				return
			} else {
				panic(fmt.Sprintf("Unexpected error: %s\n", err))
			}
		}
		evchan <- evs
	}
}

func (g *Gamepad) run() {
	log.Printf("Running...\n")
	evchan := make(chan []evdev.InputEvent, 10)
	go g.runDevice(evchan)
	for {
		select {
		case s := <-g.subChan:
			g.addSubscriber(s)
		case i := <-g.stopChan:
			g.removeSubscriber(i)
		case evs := <-evchan:
			for _, s := range g.subs {
				// Non-blocking send. Receivers who don't listen
				// get dropped!
				for _, e := range evs {
					select {
					case s.events <- e:
					default:
					}
				}
			}
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

func (g *Gamepad) initEvdev() error {
	// Wait for all the children to get probed
	// FIXME: Be more clever
	// To use udev we'd need to monitor _and_ enumerate which seems silly
	time.Sleep(2 * time.Second)

	events, err :=	filepath.Glob(g.sysdir + "/input/*/event*")
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return fmt.Errorf("Device has no input devices")
	}

	for _, p := range events {
		devnode := "/dev/input/" + filepath.Base(p)
		evdev, err := evdev.Open(devnode)
		if err != nil {
			log.Println(err)
			continue
		}
		if mainDevRegexp.Match([]byte(evdev.Name)) {
			g.evdev = evdev
			return nil
		}
		evdev.File.Close()
	}

	return fmt.Errorf("Couldn't find event device")
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

	err = g.initEvdev()
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

	g.mutex.Lock()
	defer g.mutex.Unlock()
	if g.stopped {
		return nil
	}

	g.subChan <- &s
	g.subid++

	return s.events
}

type RumbleEffect evdev.FFEffect

func(g *Gamepad) CreateRumbleEffect(strongMag, weakMag float32, duration time.Duration) (gamepad.RumbleEffect, error) {
	effect, err := g.evdev.CreateFFRumbleEffect(strongMag, weakMag, duration)
	if err != nil {
		return nil, err
	}

	return effect, nil
}

/*
func (r RumbleEffect) Play() {
	r.Play()
}
func (r RumbleEffect) Stop() {
	r.dev.StopFFEffect(r.effect)
}
func (r RumbleEffect) Delete() {
	err := r.dev.DeleteFFEffect(r.effect)
	if err != nil {
		log.Printf("Couldn't delete effect %d\n", r.effect)
	}
}
*/
