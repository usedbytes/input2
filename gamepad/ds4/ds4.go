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
	"github.com/usedbytes/linux-led"
	"github.com/usedbytes/battery"
)

var mainDevRegexp = regexp.MustCompile("Wireless Controller$")

type subscriber struct {
	id int
	stop <-chan bool
	die chan bool
	events chan input2.InputEvent
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

	filters []*input2.EventFilter

	led led.RGBLED
	battery battery.Battery
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
	if d.Action() != "remove" {
		return false
	}

	if !MatchDevice(g.device, d) {
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
			for _, e := range evs {
				var ev input2.InputEvent
				var done bool
				for _, f := range g.filters {
					if (f.Match.TypeMask & (1 << uint32(e.Type))) == 0 {
						continue
					}
					ev, done = f.Filter(&e)
					if done {
						break
					}
				}
				if ev == nil {
					continue
				}
				for _, s := range g.subs {
					// Non-blocking send. Receivers who don't listen
					// get dropped!
					select {
					case s.events <- ev:
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

func (g *Gamepad) initBattery() error {
	psus, err := filepath.Glob(g.sysdir + "/power_supply/*")
	if err != nil {
		return err
	}
	if len(psus) != 1 {
		return fmt.Errorf("Wrong number of power_supply-s")
	}
	g.battery, err = battery.NewBattery(psus[0])
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
		filters: make([]*input2.EventFilter, 0),
	}

	err := g.initUdev()
	if err != nil {
		log.Print(err)
		return nil
	}

	// Wait for all the children to get probed
	// FIXME: Be more clever
	// To use udev we'd need to monitor _and_ enumerate which seems silly
	time.Sleep(1 * time.Second)

	err = g.initEvdev()
	if err != nil {
		log.Print(err)
		return nil
	}

	err = g.initLeds()
	if err != nil {
		log.Print(err)
		return nil
	}

	err = g.initBattery()
	if err != nil {
		log.Print(err)
		return nil
	}

	go g.run()

	return g
}

func (g *Gamepad) Subscribe(stop <-chan bool) <-chan input2.InputEvent {
	s := subscriber{
		id: g.subid,
		stop: stop,
		die: make(chan bool),
		events: make(chan input2.InputEvent, 10),
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

func (g *Gamepad) AddFilter(filter *input2.EventFilter) {
	g.filters = append(g.filters, filter)
}

func (g *Gamepad) CreateRumbleEffect(strongMag, weakMag float32, duration time.Duration) (gamepad.RumbleEffect, error) {
	effect, err := g.evdev.CreateFFRumbleEffect(strongMag, weakMag, duration)
	if err != nil {
		return nil, err
	}

	return effect, nil
}

func (g *Gamepad) GetBattery() battery.Battery {
	return g.battery
}

func (g *Gamepad) Charge() float32 {
	return g.battery.Charge()
}

func (g *Gamepad) Status() battery.Status {
	return g.battery.Status()
}
