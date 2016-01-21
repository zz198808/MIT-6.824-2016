package labrpc

import "testing"
import "strconv"
import "sync"
import "runtime"
import "time"

type JunkServer struct {
	mu   sync.Mutex
	log1 []string
	log2 []int
}

func (js *JunkServer) Handler1(args string, reply *int) {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.log1 = append(js.log1, args)
	*reply, _ = strconv.Atoi(args)
}

func (js *JunkServer) Handler2(args int, reply *string) {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.log2 = append(js.log2, args)
	*reply = "handler2-" + strconv.Itoa(args)
}

func TestBasic(t *testing.T) {
	runtime.GOMAXPROCS(4)

	rn := MakeNetwork()

	e := rn.MakeEnd("end1-99")

	js := &JunkServer{}
	svc := MakeService(js)

	rs := MakeServer()
	rs.AddService(svc)
	rn.AddServer("server99", rs)

	rn.Connect("end1-99", "server99")
	rn.Enable("end1-99", true)

	{
		reply := ""
		e.Call("JunkServer.Handler2", 111, &reply)
		if reply != "handler2-111" {
			t.Fatalf("wrong reply from Handler2")
		}
	}

	{
		reply := 0
		e.Call("JunkServer.Handler1", "9099", &reply)
		if reply != 9099 {
			t.Fatalf("wrong reply from Handler1")
		}
	}
}

//
// does net.Enable(endname, false) really disconnect a client?
//
func TestDisconnect(t *testing.T) {
	runtime.GOMAXPROCS(4)

	rn := MakeNetwork()

	e := rn.MakeEnd("end1-99")

	js := &JunkServer{}
	svc := MakeService(js)

	rs := MakeServer()
	rs.AddService(svc)
	rn.AddServer("server99", rs)

	rn.Connect("end1-99", "server99")

	{
		reply := ""
		e.Call("JunkServer.Handler2", 111, &reply)
		if reply != "" {
			t.Fatalf("unexpected reply from Handler2")
		}
	}

	rn.Enable("end1-99", true)

	{
		reply := 0
		e.Call("JunkServer.Handler1", "9099", &reply)
		if reply != 9099 {
			t.Fatalf("wrong reply from Handler1")
		}
	}
}

//
// test net.GetCount()
//
func TestCounts(t *testing.T) {
	runtime.GOMAXPROCS(4)

	rn := MakeNetwork()

	e := rn.MakeEnd("end1-99")

	js := &JunkServer{}
	svc := MakeService(js)

	rs := MakeServer()
	rs.AddService(svc)
	rn.AddServer(99, rs)

	rn.Connect("end1-99", 99)
	rn.Enable("end1-99", true)

	for i := 0; i < 17; i++ {
		reply := ""
		e.Call("JunkServer.Handler2", i, &reply)
		wanted := "handler2-" + strconv.Itoa(i)
		if reply != wanted {
			t.Fatalf("wrong reply %v from Handler1, expecting %v", reply, wanted)
		}
	}

	n := rn.GetCount(99)
	if n != 17 {
		t.Fatalf("wrong GetCount() %v, expected 17\n", n)
	}
}

//
// test RPCs from concurrent ClientEnds
//
func TestConcurrentMany(t *testing.T) {
	runtime.GOMAXPROCS(4)

	rn := MakeNetwork()

	js := &JunkServer{}
	svc := MakeService(js)

	rs := MakeServer()
	rs.AddService(svc)
	rn.AddServer(1000, rs)

	ch := make(chan int)

	nclients := 20
	nrpcs := 10
	for ii := 0; ii < nclients; ii++ {
		go func(i int) {
			n := 0
			defer func() { ch <- n }()

			e := rn.MakeEnd(i)
			rn.Connect(i, 1000)
			rn.Enable(i, true)

			for j := 0; j < nrpcs; j++ {
				arg := i*100 + j
				reply := ""
				e.Call("JunkServer.Handler2", arg, &reply)
				wanted := "handler2-" + strconv.Itoa(arg)
				if reply != wanted {
					t.Fatalf("wrong reply %v from Handler1, expecting %v", reply, wanted)
				}
				n += 1
			}
		}(ii)
	}

	total := 0
	for ii := 0; ii < nclients; ii++ {
		x := <-ch
		total += x
	}

	if total != nclients*nrpcs {
		t.Fatalf("wrong number of RPCs completed, got %v, expected %v", total, nclients*nrpcs)
	}

	n := rn.GetCount(1000)
	if n != total {
		t.Fatalf("wrong GetCount() %v, expected %v\n", n, total)
	}
}

//
// test unreliable
//
func TestUnreliable(t *testing.T) {
	runtime.GOMAXPROCS(4)

	rn := MakeNetwork()
	rn.Reliable(false)

	js := &JunkServer{}
	svc := MakeService(js)

	rs := MakeServer()
	rs.AddService(svc)
	rn.AddServer(1000, rs)

	ch := make(chan int)

	nclients := 300
	for ii := 0; ii < nclients; ii++ {
		go func(i int) {
			n := 0
			defer func() { ch <- n }()

			e := rn.MakeEnd(i)
			rn.Connect(i, 1000)
			rn.Enable(i, true)

			arg := i * 100
			reply := ""
			ok := e.Call("JunkServer.Handler2", arg, &reply)
			if ok {
				wanted := "handler2-" + strconv.Itoa(arg)
				if reply != wanted {
					t.Fatalf("wrong reply %v from Handler1, expecting %v", reply, wanted)
				}
				n += 1
			}
		}(ii)
	}

	total := 0
	for ii := 0; ii < nclients; ii++ {
		x := <-ch
		total += x
	}

	if total == nclients || total == 0 {
		t.Fatalf("all RPCs succeeded despite unreliable")
	}
}

//
// test concurrent RPCs from a single ClientEnd
//
func TestConcurrentOne(t *testing.T) {
	runtime.GOMAXPROCS(4)

	rn := MakeNetwork()

	js := &JunkServer{}
	svc := MakeService(js)

	rs := MakeServer()
	rs.AddService(svc)
	rn.AddServer(1000, rs)

	e := rn.MakeEnd("c")
	rn.Connect("c", 1000)
	rn.Enable("c", true)

	ch := make(chan int)

	nrpcs := 20
	for ii := 0; ii < nrpcs; ii++ {
		go func(i int) {
			n := 0
			defer func() { ch <- n }()

			arg := 100 + i
			reply := ""
			e.Call("JunkServer.Handler2", arg, &reply)
			wanted := "handler2-" + strconv.Itoa(arg)
			if reply != wanted {
				t.Fatalf("wrong reply %v from Handler2, expecting %v", reply, wanted)
			}
			n += 1
		}(ii)
	}

	total := 0
	for ii := 0; ii < nrpcs; ii++ {
		x := <-ch
		total += x
	}

	if total != nrpcs {
		t.Fatalf("wrong number of RPCs completed, got %v, expected %v", total, nrpcs)
	}

	js.mu.Lock()
	defer js.mu.Unlock()
	if len(js.log2) != nrpcs {
		t.Fatalf("wrong number of RPCs delivered")
	}

	n := rn.GetCount(1000)
	if n != total {
		t.Fatalf("wrong GetCount() %v, expected %v\n", n, total)
	}
}

//
// regression: an RPC that's delayed during Enabled=false
// should not delay subsequent RPCs (e.g. after Enabled=true).
//
func TestRegression1(t *testing.T) {
	runtime.GOMAXPROCS(4)

	rn := MakeNetwork()

	js := &JunkServer{}
	svc := MakeService(js)

	rs := MakeServer()
	rs.AddService(svc)
	rn.AddServer(1000, rs)

	e := rn.MakeEnd("c")
	rn.Connect("c", 1000)

	// start some RPCs while the ClientEnd is disabled.
	// they'll be delayed.
	rn.Enable("c", false)
	ch := make(chan bool)
	nrpcs := 20
	for ii := 0; ii < nrpcs; ii++ {
		go func(i int) {
			ok := false
			defer func() { ch <- ok }()

			arg := 100 + i
			reply := ""
			// this call ought to return false.
			e.Call("JunkServer.Handler2", arg, &reply)
			ok = true
		}(ii)
	}

	time.Sleep(100 * time.Millisecond)

	// now enable the ClientEnd and check that an RPC completes quickly.
	t0 := time.Now()
	rn.Enable("c", true)
	{
		arg := 99
		reply := ""
		e.Call("JunkServer.Handler2", arg, &reply)
		wanted := "handler2-" + strconv.Itoa(arg)
		if reply != wanted {
			t.Fatalf("wrong reply %v from Handler2, expecting %v", reply, wanted)
		}
	}
	dur := time.Since(t0).Seconds()

	if dur > 0.03 {
		t.Fatalf("RPC took too long (%v) after Enable", dur)
	}

	for ii := 0; ii < nrpcs; ii++ {
		<-ch
	}

	js.mu.Lock()
	defer js.mu.Unlock()
	if len(js.log2) != 1 {
		t.Fatalf("wrong number (%v) of RPCs delivered, expected 1", len(js.log2))
	}

	n := rn.GetCount(1000)
	if n != 1 {
		t.Fatalf("wrong GetCount() %v, expected %v\n", n, 1)
	}
}