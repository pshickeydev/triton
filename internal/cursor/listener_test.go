package cursor

import (
	"net"
	"testing"
	"time"

	"github.com/hypebeast/go-osc/osc"
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
