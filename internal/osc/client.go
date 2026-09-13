package osc

import (
	"fmt"

	"github.com/hypebeast/go-osc/osc"

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

type Client struct {
	client *osc.Client
}

func New(host string, port int) *Client {
	return &Client{
		client: osc.NewClient(host, port),
	}
}

func (c *Client) Hush() error {
	return c.client.Send(osc.NewMessage(addrHush))
}

func (c *Client) MuteAll() error {
	return c.client.Send(osc.NewMessage(addrMuteAll))
}

func (c *Client) UnmuteAll() error {
	return c.client.Send(osc.NewMessage(addrUnmuteAll))
}

func (c *Client) Mute(streamNum int) error {
	if err := validateStream(streamNum); err != nil {
		return err
	}
	return c.client.Send(osc.NewMessage(addrMute, int32(streamNum)))
}

func (c *Client) Unmute(streamNum int) error {
	if err := validateStream(streamNum); err != nil {
		return err
	}
	return c.client.Send(osc.NewMessage(addrUnmute, int32(streamNum)))
}

func (c *Client) Solo(streamNum int) error {
	if err := validateStream(streamNum); err != nil {
		return err
	}
	return c.client.Send(osc.NewMessage(addrSolo, int32(streamNum)))
}

func (c *Client) Unsolo(streamNum int) error {
	if err := validateStream(streamNum); err != nil {
		return err
	}
	return c.client.Send(osc.NewMessage(addrUnsolo, int32(streamNum)))
}

func (c *Client) SetTempo(cps float64) error {
	return c.client.Send(osc.NewMessage(addrSetCPS, float32(cps)))
}

func validateStream(streamNum int) error {
	if streamNum < tidal.MinStream || streamNum > tidal.MaxStream {
		return errStreamOutOfRange
	}
	return nil
}
