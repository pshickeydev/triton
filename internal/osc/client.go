// Package osc implements the transport-control side of triton's OSC usage:
// sending mute/solo/hush/tempo messages to SuperDirt.
package osc

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/pshickeydev/go-osc/osc"

	"github.com/pshickeydev/triton/internal/tidal"
)

const (
	addrHush      = "/hush"
	addrMuteAll   = "/muteAll"
	addrUnmuteAll = "/unmuteAll"
	addrMute      = "/mute"
	addrUnmute    = "/unmute"
	addrSolo      = "/solo"
	addrUnsolo    = "/unsolo"
	addrSetCPS    = "/setcps"
)

var errStreamOutOfRange = fmt.Errorf("stream number must be %d-%d", tidal.MinStream, tidal.MaxStream)

// Client sends transport-control OSC messages to SuperDirt over a single
// persistent UDP socket, rather than dialing one socket per message.
type Client struct {
	host string
	port int
	mu   sync.Mutex
	conn *net.UDPConn
}

func New(host string, port int) *Client {
	return &Client{host: host, port: port}
}

func (c *Client) Hush() error {
	return c.send(osc.NewMessage(addrHush))
}

func (c *Client) MuteAll() error {
	return c.send(osc.NewMessage(addrMuteAll))
}

func (c *Client) UnmuteAll() error {
	return c.send(osc.NewMessage(addrUnmuteAll))
}

func (c *Client) Mute(streamNum int) error {
	if err := validateStream(streamNum); err != nil {
		return err
	}
	return c.send(osc.NewMessage(addrMute, int32(streamNum)))
}

func (c *Client) Unmute(streamNum int) error {
	if err := validateStream(streamNum); err != nil {
		return err
	}
	return c.send(osc.NewMessage(addrUnmute, int32(streamNum)))
}

func (c *Client) Solo(streamNum int) error {
	if err := validateStream(streamNum); err != nil {
		return err
	}
	return c.send(osc.NewMessage(addrSolo, int32(streamNum)))
}

func (c *Client) Unsolo(streamNum int) error {
	if err := validateStream(streamNum); err != nil {
		return err
	}
	return c.send(osc.NewMessage(addrUnsolo, int32(streamNum)))
}

func (c *Client) SetTempo(cps float64) error {
	return c.send(osc.NewMessage(addrSetCPS, float32(cps)))
}

// send marshals the message and writes it on the socket, dialing on first
// use and re-dialing on the next send after a failed write.
func (c *Client) send(msg *osc.Message) error {
	conn, err := c.socket()
	if err != nil {
		return err
	}
	data, err := msg.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err := conn.Write(data); err != nil {
		c.drop()
		return err
	}
	return nil
}

func (c *Client) socket() (*net.UDPConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn, nil
	}
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(c.host, strconv.Itoa(c.port)))
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

func (c *Client) drop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.closeLocked()
}

// closeLocked closes the socket; the caller holds the mutex.
func (c *Client) closeLocked() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

// Close releases the socket. It may be skipped at exit; the OS closes the
// socket with the process.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeLocked()
}

func validateStream(streamNum int) error {
	if streamNum < tidal.MinStream || streamNum > tidal.MaxStream {
		return errStreamOutOfRange
	}
	return nil
}
