package osc

import (
	"net"
	"testing"
	"time"

	"github.com/pshickeydev/go-osc/osc"
)

func captureMessage(t *testing.T, send func(*Client) error) *osc.Message {
	t.Helper()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening for OSC packets: %v", err)
	}
	defer conn.Close()

	c := New("127.0.0.1", conn.LocalAddr().(*net.UDPAddr).Port)

	go func() {
		_ = send(c)
	}()

	data := make([]byte, 65535)
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("setting read deadline: %v", err)
	}
	n, _, err := conn.ReadFrom(data)
	if err != nil {
		t.Fatalf("reading OSC packet: %v", err)
	}

	pkt, err := osc.ParsePacket(string(data[:n]))
	if err != nil {
		t.Fatalf("parsing OSC packet: %v", err)
	}
	msg, ok := pkt.(*osc.Message)
	if !ok {
		t.Fatalf("expected an OSC message, got %T", pkt)
	}
	return msg
}

func assertStreamArg(t *testing.T, msg *osc.Message, addr string, want int32) {
	t.Helper()
	if msg.Address != addr {
		t.Errorf("expected address %q, got %q", addr, msg.Address)
	}
	if len(msg.Arguments) != 1 {
		t.Fatalf("expected 1 argument for %s, got %d", addr, len(msg.Arguments))
	}
	got, ok := msg.Arguments[0].(int32)
	if !ok {
		t.Fatalf("expected int32 argument for %s, got %T", addr, msg.Arguments[0])
	}
	if got != want {
		t.Errorf("expected %s argument %d, got %d", addr, want, got)
	}
}

func TestMute_Wire(t *testing.T) {
	msg := captureMessage(t, func(c *Client) error { return c.Mute(1) })
	assertStreamArg(t, msg, "/mute", 1)
}

func TestUnmute_Wire(t *testing.T) {
	msg := captureMessage(t, func(c *Client) error { return c.Unmute(8) })
	assertStreamArg(t, msg, "/unmute", 8)
}

func TestSolo_Wire(t *testing.T) {
	msg := captureMessage(t, func(c *Client) error { return c.Solo(3) })
	assertStreamArg(t, msg, "/solo", 3)
}

func TestUnsolo_Wire(t *testing.T) {
	msg := captureMessage(t, func(c *Client) error { return c.Unsolo(16) })
	assertStreamArg(t, msg, "/unsolo", 16)
}

func TestSetTempo_Wire(t *testing.T) {
	msg := captureMessage(t, func(c *Client) error { return c.SetTempo(1.35) })
	if msg.Address != "/setcps" {
		t.Errorf("expected address %q, got %q", "/setcps", msg.Address)
	}
	if len(msg.Arguments) != 1 {
		t.Fatalf("expected 1 argument for /setcps, got %d", len(msg.Arguments))
	}
	got, ok := msg.Arguments[0].(float32)
	if !ok {
		t.Fatalf("expected float32 argument for /setcps, got %T", msg.Arguments[0])
	}
	if got != 1.35 {
		t.Errorf("expected /setcps argument 1.35, got %f", got)
	}
}

func TestNew(t *testing.T) {
	c := New("127.0.0.1", 6010)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestMute_InvalidStream(t *testing.T) {
	c := New("127.0.0.1", 6010)
	if err := c.Mute(0); err == nil {
		t.Fatal("expected error for stream 0")
	}
	if err := c.Mute(17); err == nil {
		t.Fatal("expected error for stream 17")
	}
}

func TestUnmute_InvalidStream(t *testing.T) {
	c := New("127.0.0.1", 6010)
	if err := c.Unmute(0); err == nil {
		t.Fatal("expected error for stream 0")
	}
}

func TestSolo_InvalidStream(t *testing.T) {
	c := New("127.0.0.1", 6010)
	if err := c.Solo(0); err == nil {
		t.Fatal("expected error for stream 0")
	}
}

func TestUnsolo_InvalidStream(t *testing.T) {
	c := New("127.0.0.1", 6010)
	if err := c.Unsolo(0); err == nil {
		t.Fatal("expected error for stream 0")
	}
}

// sourcePort reads one datagram and reports the sender's port, which is how
// the test can tell whether the client reused its socket.
func sourcePort(t *testing.T, conn net.PacketConn) int {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("setting read deadline: %v", err)
	}
	buf := make([]byte, 65535)
	_, src, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("reading OSC packet: %v", err)
	}
	return src.(*net.UDPAddr).Port
}

func TestClientReusesOneSocket(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening for OSC packets: %v", err)
	}
	defer conn.Close()

	client := New("127.0.0.1", conn.LocalAddr().(*net.UDPAddr).Port)
	defer client.Close()

	if err := client.Mute(1); err != nil {
		t.Fatalf("first send: %v", err)
	}
	first := sourcePort(t, conn)

	if err := client.Mute(2); err != nil {
		t.Fatalf("second send: %v", err)
	}
	if second := sourcePort(t, conn); second != first {
		t.Errorf("client should reuse one socket, dialed again: %d -> %d", first, second)
	}

	client.Close()
	if err := client.Mute(3); err != nil {
		t.Fatalf("sending after Close should re-dial: %v", err)
	}
	if third := sourcePort(t, conn); third == first {
		t.Errorf("expected a fresh socket after Close, got port %d again", third)
	}
}
