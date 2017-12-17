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

type subscription struct{
	stop <-chan bool
	events chan<- []evdev.InputEvent
}

type connection struct{
	dev *Gamepad
	filters map[input2.EventMatch]input2.EventFilter
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

	subs map[int]chan<- []evdev.InputEvent
	subChan chan *subscription
	stopChan chan int
	die chan struct{}

	led led.RGBLED
	battery battery.Battery
}

func (g *Gamepad) addSubscriber(s *subscription) {
	id := g.subid
	g.subid += 1
	log.Printf("New subscription: %d\n", id)

	g.subs[id] = s.events
	go func(stop <-chan bool, id int) {
		select {
		case <-g.die:
		case <-stop:
		}
		g.stopChan <-id
	}(s.stop, id)
}

func (g *Gamepad) removeSubscriber(id int) {
	log.Printf("Subscription %d is done\n", id)

	evchan := g.subs[id]
	if evchan == nil {
		return
	}

	delete(g.subs, id)
	close(evchan)
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
	close(g.die)

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
	// subChan
	go func() {
		for s := range g.subChan {
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
		//fmt.Printf("Read %d evs\n", len(evs))
		/*
		select {
		case evchan <- evs:
		default:
			fmt.Printf("Events dropped!!!\n")
		}
		*/
		evchan <- evs
	}
}

func (g *Gamepad) run() {
	log.Printf("Running...\n")
	evchan := make(chan []evdev.InputEvent)
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
				select {
				case s <- evs:
				default:
				}
				/*
				for _, ev := range evs {
					match := input2.EventMatch{ ev.Type, ev.Code }
					f, ok := s.conn.filters[match]
					if !ok {
						match = input2.EventMatch{ ev.Type, input2.EvcodeAny }
						f, ok = s.conn.filters[match]
						if !ok {
							f, ok = s.conn.filters[input2.MatchAll]
						}
					}

					if ok {
						log.Printf("ev matched\n");
					}

					_ = f
					_ = ok

				}
				*/
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

func (g *Gamepad) bleh() error {
	for ct, cv := range g.evdev.Capabilities {
		if ct.Type == evdev.EV_ABS {
			for _, ax := range cv {
				fmt.Printf("Axis %d %s\n", ax.Code, ax.Name)
				info, err := g.evdev.GetAbsInfo(ax.Code)
				if err == nil {
					fmt.Printf("%#v\n", info)
				} else {
					fmt.Println(err)
				}
			}
		} else {
			fmt.Printf("Skip %s\n", ct.Name)
		}
	}

	return nil
}

func NewGamepad(sysdir string) *Gamepad {
	log.Printf("Gamepad %s\n", sysdir)
	g := &Gamepad {
		subid: 0,
		sysdir: sysdir,
		subs: make(map[int]chan<- []evdev.InputEvent),
		subChan: make(chan *subscription),
		stopChan: make(chan int, 5),
		die: make(chan struct{}),
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

	g.bleh()

	go g.run()

	return g
}

func (g *Gamepad) NewConnection() input2.Connection {
	c := connection{
		dev: g,
		filters: make(map[input2.EventMatch]input2.EventFilter),
	}

	return &c
}

// Will replace any existing filter with this match
// Shouldn't be called with current subscriptions
func (c *connection) SetFilter(match input2.EventMatch, filter input2.EventFilter) {
	c.filters[match] = filter
}

func (c *connection) run(tx chan<- input2.InputEvent, rx <-chan []evdev.InputEvent) {
	for evs := range rx {
		syncs := make(map[input2.SyncFilter]struct{})
		for _, ev := range evs {
			if ev.Type == evdev.EV_SYN {
				for k, _ := range syncs {
					k.Sync(ev, tx)
				}
				continue
			}

			match := input2.EventMatch{ ev.Type, ev.Code }
			f, ok := c.filters[match]
			if ok {
				if sync, ok := f.(input2.SyncFilter); ok {
					syncs[sync] = struct{}{}
				}
				f.Filter(ev, tx)
			}
		}
	}
	close(tx)
}

func (c *connection) Subscribe(stop <-chan bool) <-chan input2.InputEvent {
	tx := make(chan input2.InputEvent, 10)
	rx := make(chan []evdev.InputEvent, 10)
	go c.run(tx, rx)

	c.dev.mutex.Lock()
	defer c.dev.mutex.Unlock()
	if c.dev.stopped {
		return nil
	}

	c.dev.subChan <- &subscription{ stop, rx }

	return tx
}

func (g *Gamepad) CreateRumbleEffect(strongMag, weakMag float32, duration time.Duration) (gamepad.RumbleEffect, error) {
	effect, err := g.evdev.CreateFFRumbleEffect(strongMag, weakMag, duration)
	if err != nil {
		return nil, err
	}

	return effect, nil
}
