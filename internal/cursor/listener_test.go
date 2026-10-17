package cursor

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/pshickeydev/go-osc/osc"
)

// freePort grabs an available UDP port for the listener to bind.
func freePort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

// startListener runs the listener in the background and waits for its
// socket to come up before the test sends to it.
func startListener(t *testing.T, l *Listener) {
	t.Helper()
	go func() {
		_ = l.Listen()
	}()
	time.Sleep(200 * time.Millisecond)
}

func TestListenerDecodesHighlights(t *testing.T) {
	port := freePort(t)
	l := NewListener("127.0.0.1", port)
	startListener(t, l)

	client := osc.NewClient("127.0.0.1", port)
	// OSC args carry 0-based [startCol, startLine, endCol, endLine).
	msg := osc.NewMessage(highlightsAddr)
	msg.Append("1")
	msg.Append(float32(123456))
	msg.Append(float32(0.5))
	msg.Append(int32(12))
	msg.Append(int32(3))
	msg.Append(int32(15))
	msg.Append(int32(3))
	if err := client.Send(msg); err != nil {
		t.Fatal(err)
	}

	select {
	case hl := <-l.Highlights():
		if hl.Stream != 1 {
			t.Errorf("stream: got %d, want 1", hl.Stream)
		}
		// 0-based [12,15) converts to 1-based [13,16).
		if hl.Line != 3 || hl.EndLine != 3 || hl.StartCol != 13 || hl.EndCol != 16 {
			t.Errorf("position: got %+v", hl)
		}
		if hl.Cycle != 0.5 {
			t.Errorf("cycle: got %v, want 0.5", hl.Cycle)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no highlight received")
	}
}

func TestListenerCloseStopsListen(t *testing.T) {
	port := freePort(t)
	l := NewListener("127.0.0.1", port)
	done := make(chan error, 1)
	go func() {
		done <- l.Listen()
	}()
	time.Sleep(200 * time.Millisecond)

	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Listen should return nil after Close, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Listen did not return after Close")
	}
}

func TestListenerIgnoresOtherPatterns(t *testing.T) {
	port := freePort(t)
	l := NewListener("127.0.0.1", port)
	startListener(t, l)

	client := osc.NewClient("127.0.0.1", port)
	for _, ident := range []string{"foo", "17", "0"} {
		msg := osc.NewMessage(highlightsAddr)
		msg.Append(ident)
		msg.Append(float32(0))
		msg.Append(float32(0))
		msg.Append(int32(1))
		msg.Append(int32(1))
		msg.Append(int32(2))
		msg.Append(int32(1))
		if err := client.Send(msg); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case hl := <-l.Highlights():
		t.Fatalf("pattern id should be ignored, got stream %d", hl.Stream)
	case <-time.After(100 * time.Millisecond):
	}
}

// highlightMsg builds one decoded-ready /editor/highlights message for the
// given stream and 0-based span.
func highlightMsg(ident string, startCol, startLine, endCol, endLine int32) *osc.Message {
	msg := osc.NewMessage(highlightsAddr)
	msg.Append(ident)
	msg.Append(float32(0))
	msg.Append(float32(1))
	msg.Append(startCol)
	msg.Append(startLine)
	msg.Append(endCol)
	msg.Append(endLine)
	return msg
}

// Tidal schedules the editor target with bundle stamps, so highlights arrive
// packed in a bundle rather than one datagram each.
func TestListenerDecodesBundles(t *testing.T) {
	port := freePort(t)
	l := NewListener("127.0.0.1", port)
	startListener(t, l)

	bundle := osc.NewBundle(time.Now())
	if err := bundle.Append(highlightMsg("1", 12, 3, 15, 3)); err != nil {
		t.Fatal(err)
	}
	if err := bundle.Append(osc.NewMessage("/other/address")); err != nil {
		t.Fatal(err)
	}
	inner := osc.NewBundle(time.Now())
	if err := inner.Append(highlightMsg("2", 1, 0, 4, 0)); err != nil {
		t.Fatal(err)
	}
	if err := bundle.Append(inner); err != nil {
		t.Fatal(err)
	}

	client := osc.NewClient("127.0.0.1", port)
	if err := client.Send(bundle); err != nil {
		t.Fatal(err)
	}

	for _, want := range []int{1, 2} {
		select {
		case hl := <-l.Highlights():
			if hl.Stream != want {
				t.Fatalf("stream: got %d, want %d (%+v)", hl.Stream, want, hl)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for stream %d from the bundle", want)
		}
	}
}

// A malformed datagram must not stop the listener: later highlights still
// arrive.
func TestListenerDropsMalformedDatagram(t *testing.T) {
	port := freePort(t)
	l := NewListener("127.0.0.1", port)
	startListener(t, l)

	conn, err := net.Dial("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("not an osc packet")); err != nil {
		t.Fatal(err)
	}

	client := osc.NewClient("127.0.0.1", port)
	if err := client.Send(highlightMsg("1", 12, 3, 15, 3)); err != nil {
		t.Fatal(err)
	}

	select {
	case hl := <-l.Highlights():
		if hl.Stream != 1 {
			t.Errorf("stream: got %d, want 1", hl.Stream)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener stopped after a malformed datagram")
	}
}
