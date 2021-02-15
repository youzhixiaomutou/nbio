// +build windows

package nbio

import (
	"runtime"
	"strings"
	"time"

	"github.com/lesismal/nbio/log"
)

// Start init and start pollers
func (g *Gopher) Start() error {
	var err error

	g.lfds = []int{}

	g.listeners = make([]*poller, len(g.addrs))
	for i := range g.addrs {
		g.listeners[i], err = newPoller(g, true, int(i))
		if err != nil {
			for j := 0; j < i; j++ {
				g.listeners[j].stop()
			}
			return err
		}
	}

	for i := uint32(0); i < g.pollerNum; i++ {
		g.pollers[i], err = newPoller(g, false, int(i))
		if err != nil {
			for j := 0; j < len(g.addrs); j++ {
				g.listeners[j].stop()
			}

			for j := 0; j < int(i); j++ {
				g.pollers[j].stop()
			}
			return err
		}
	}

	for i := uint32(0); i < g.pollerNum; i++ {
		g.Add(1)
		go g.pollers[i].start()
	}
	for _, l := range g.listeners {
		g.Add(1)
		go l.start()
	}

	g.Add(1)
	go g.timerLoop()

	if len(g.addrs) == 0 {
		log.Info("Gopher[%v] start", g.Name)
	} else {
		log.Info("Gopher[%v] start listen on: [\"%v\"]", g.Name, strings.Join(g.addrs, `", "`))
	}
	return nil
}

// NewGopher is a factory impl
func NewGopher(conf Config) *Gopher {
	cpuNum := uint32(runtime.NumCPU())
	if conf.Name == "" {
		conf.Name = "NB"
	}
	if conf.MaxLoad == 0 {
		conf.MaxLoad = DefaultMaxLoad
	}
	if len(conf.Addrs) > 0 && conf.NListener == 0 {
		conf.NListener = 1
	}
	if conf.NPoller == 0 {
		conf.NPoller = cpuNum
	}
	if conf.ReadBufferSize == 0 {
		conf.ReadBufferSize = DefaultReadBufferSize
	}

	g := &Gopher{
		Name:               conf.Name,
		network:            conf.Network,
		addrs:              conf.Addrs,
		maxLoad:            int64(conf.MaxLoad),
		listenerNum:        conf.NListener,
		pollerNum:          conf.NPoller,
		readBufferSize:     conf.ReadBufferSize,
		maxWriteBufferSize: conf.MaxWriteBufferSize,
		listeners:          make([]*poller, conf.NListener),
		pollers:            make([]*poller, conf.NPoller),
		connsStd:           map[*Conn][]byte{},
		// connsUnix:         make([]*Conn, conf.MaxLoad+64),
		onOpen:  func(c *Conn) {},
		onClose: func(c *Conn, err error) {},
		onData:  func(c *Conn, data []byte) {},
		trigger: time.NewTimer(timeForever),
		chTimer: make(chan struct{}),
	}

	g.onMemAlloc = func(c *Conn) []byte {
		g.mux.Lock()
		buf, ok := g.connsStd[c]
		if !ok {
			buf = make([]byte, g.readBufferSize)
			g.connsStd[c] = buf
		}
		g.mux.Unlock()
		return buf
	}

	return g
}
