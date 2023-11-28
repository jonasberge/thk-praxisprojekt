package container

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// FIXME while this is a mess and deserves a rewrite, it allows us
//  to run test functions within different containers to test network operations.

const (
	Network      = "tcp"
	Port         = 10000
	RetryAfter   = 100 * time.Millisecond
	WriteTimeout = 1 * time.Second
	ReadTimeout  = 1 * time.Second
	// AfterDelay is added after each Container.Run execution
	// in order to give network operations time to finish.
	// This is not an elegant solution but it works fine for now.
	AfterDelay = 10 * time.Millisecond
	ReadyByte  = byte(0xFF)
)

type TestFunc = func(*testing.T)

type request struct {
	fun      string
	response chan<- bool
}

type waitRequest struct {
	fun      string
	response chan<- bool
}

type Container struct {
	host     string
	port     int
	listens  int32
	ready    chan string
	requests chan request
	waitFor  chan waitRequest
	runDone  chan struct{}
	reset    chan bool
}

func New(hostname string) *Container {
	return &Container{
		host:     hostname,
		port:     Port,
		listens:  0,
		ready:    make(chan string),
		requests: make(chan request),
		waitFor:  make(chan waitRequest),
		runDone:  nil,
		reset:    make(chan bool),
	}
}

func (c *Container) listen() {
	listener, err := net.Listen(Network, c.Addr())
	if err != nil {
		failed("to listen:", err)
	}
	go c.handleRequests()
	for {
		var conn net.Conn
		conn, err = listener.Accept()
		if err != nil {
			failed("to accept a connection:", err)
		}
		go c.handleIncoming(conn)
	}
}

func (c *Container) handleIncoming(conn net.Conn) {
	defer conn.Close()
	err := conn.SetReadDeadline(time.Now().Add(ReadTimeout))
	if err != nil {
		failed("to set read deadline:", err)
	}
	req := make([]byte, 256)
	n, err := conn.Read(req)
	if err == io.EOF && n == 0 {
		return
	}
	if err != nil {
		failed("to read request:", err)
	}
	fun := string(req[:n])
	ready := make(chan bool)
	c.requests <- request{fun, ready}
	ok := <-ready
	if !ok {
		fail("invalid ready value")
	}
	err = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	if err != nil {
		failed("to set write deadline:", err)
	}
	_, err = conn.Write([]byte{ReadyByte})
	if err != nil {
		failed("to write ready status:", err)
	}
}

func (c *Container) handleRequests() {
	const Size = 16
	type yes struct{}
	ready := make(map[string]yes, Size)
	completed := make(map[string]yes, Size)
	requests := make(map[string]chan<- bool, Size)
	waits := make(map[string]chan<- bool)
	done := func(fun string, response chan<- bool) {
		response <- true
		completed[fun] = yes{}
	}
	for {
		select {
		case <-c.reset:
			// FIXME: this is violent
			ready = make(map[string]yes, Size)
			completed = make(map[string]yes, Size)
			requests = make(map[string]chan<- bool, Size)
			waits = make(map[string]chan<- bool)
		case fun := <-c.ready:
			if _, ok := ready[fun]; ok {
				// fail("cannot wait twice in the same function")
				// continue
			}
			ready[fun] = yes{}
			if response, ok := requests[fun]; ok {
				done(fun, response)
				delete(requests, fun)
			}
		case req := <-c.requests:
			if _, ok := completed[req.fun]; ok {
				fail("received a requests for a function that has already been completed")
			}
			if waitRes, ok := waits[req.fun]; ok {
				waitRes <- true
				delete(waits, req.fun)
			}
			if _, ok := ready[req.fun]; ok {
				done(req.fun, req.response)
			} else {
				requests[req.fun] = req.response
			}
		case waitReq := <-c.waitFor:
			if _, ok := waits[waitReq.fun]; ok {
				panic("received multiple wait requests for the same function")
			}
			waits[waitReq.fun] = waitReq.response
		}
	}
}

func (c *Container) wait(other *Container, dstFuncName string, srcFuncName string) {
	if atomic.CompareAndSwapInt32(&c.listens, 0, 1) {
		// start listening once this function is called
		// because we do not want the user to explicitly have to call listen
		// and calling listen in the constructor will bind the port multiple times
		// since multiple containers are instantiated within the same test file.
		go c.listen()
	}
	if c.Addr() == other.Addr() {
		panic("waiting for the same container is forbidden")
	}
	c.ready <- srcFuncName
	contacted := make(chan bool)
	requested := make(chan bool)
	c.waitFor <- waitRequest{srcFuncName, requested}
	var done uint32 = 0
	go func() {
		for {
			conn, err := net.Dial(Network, other.Addr())
			if err != nil {
				time.Sleep(RetryAfter)
				continue
			}
			if atomic.LoadUint32(&done) == 1 {
				conn.Close()
				return
			}
			c.handleOutgoing(conn, dstFuncName)
			contacted <- true
			return
		}
	}()
	select {
	case <-contacted:
	case <-requested:
	}
	atomic.StoreUint32(&done, 1)
}

func (c *Container) handleOutgoing(conn net.Conn, dst string) {
	defer conn.Close()
	err := conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	if err != nil {
		failed("to set write deadline:", err)
	}
	req := []byte(dst)
	_, err = conn.Write(req)
	if err != nil {
		failed("to write request to connection", err)
	}
	res := make([]byte, 1)
	n, err := conn.Read(res)
	if err == io.EOF && n == 0 {
		return
	}
	if err != nil {
		failed("to read response:", err)
	}
	if res[0] != ReadyByte {
		fail(fmt.Sprintf("received invalid response: %v (%v)", res, n))
	}
}

// WaitFor waits until another container is executing the function dstFunc
// and has called WaitFor with the calling container and the name of the calling function as an argument.
func (c *Container) WaitFor(other *Container, dstFunc interface{}) {
	c.wait(other, funcName(dstFunc), callerFuncName())
}

// Sync synchronizes execution of the function in which the group was instantiated
// between the containers of this group.
func (c *Container) Sync(group *Group) {
	for _, other := range group.Containers {
		if c != other {
			c.wait(other, group.CallerFunc, group.CallerFunc)
		}
	}
}

// Run runs the given function in the container on which this method was called.
// If the function is run is determined by the configured hostname of the container
// and the actual hostname of the device this program is running on.
// Use RunAsync the function starts goroutines that might outlive the lifetime of this function.
func (c *Container) Run(fun func()) {
	if c.canRun() {
		fun()
		time.Sleep(AfterDelay)
		c.Reset()
	}
}

// RunAsync runs the given function just like Run
// but allows the use of goroutines that might outlive the lifetime of this function.
// A call to Done marks the end of this async execution.
func (c *Container) RunAsync(fun func()) {
	if c.canRun() {
		c.runDone = make(chan struct{})
		fun()
		<-c.runDone
		time.Sleep(AfterDelay)
		c.Reset()
	}
}

// TODO Create RunAsyncGroup() that allows usage of multiple goroutines where a sync.WaitGroup is used.

func (c *Container) canRun() bool {
	return ipOf(c.Host()).Equal(ipOf(Hostname()))
}

// Done ends the execution of an async context that was started via RunAsync.
func (c *Container) Done() {
	if c.runDone == nil {
		fail("Done may only be called in a RunAsync context")
	}
	close(c.runDone)
}

func (c *Container) Reset() {
	c.reset <- true
}

// Host returns the configured hostname of this container.
// To get the actual hostname call Hostname.
func (c *Container) Host() string {
	return c.host
}

func (c *Container) HasIP(ip net.IP) bool {
	ips, err := net.LookupIP(c.Host())
	if err != nil {
		failed("to lookup the host's ip addresses:", err)
	}
	for _, i := range ips {
		if i.To4() != nil && i.Equal(ip) {
			return true
		}
	}
	return false
}

// Addr returns the address of the TCP endpoint that is used for synchronization.
func (c *Container) Addr() string {
	return net.JoinHostPort(c.host, strconv.Itoa(c.port))
}

// Hostname returns the actual hostname of the container on which this program is running.
func Hostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		failed("to retrieve hostname:", err)
	}
	return hostname
}
