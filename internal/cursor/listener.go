// Package cursor implements the pattern tracing cursor's inbound side: an
// OSC listener for Tidal's OSCContext messages and the highlight span state
// derived from them.
//
// When booted with boot/BootTidal-triton.hs, Tidal sends one
// /editor/highlights message per mininotation fragment of every event, on
// every clock tick, for each event intersecting the tick arc. The messages
// carry [ident, delta, cycle, startCol, startLine, endCol, endLine] where
// ident is the pattern id ("1".."16" for d1..d16). After the deltamini
// rewrite at evaluation time, the columns are 0-based [start, end) buffer
// indices and the lines 1-based buffer lines; the listener converts the
// columns to the 1-based convention used by the rest of the TUI.
package cursor

import (
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/pshickeydev/go-osc/osc"

	"github.com/pshickeydev/triton/internal/tidal"
)

const (
	highlightsAddr = "/editor/highlights"
	numMsgArgs     = 7
	chanBuffer     = 256
)

// Highlight is one decoded source-position message from Tidal.
type Highlight struct {
	Stream   int     // 1-indexed d-stream
	Cycle    float64 // cycle position of the event
	Line     int     // 1-based buffer line, inclusive
	EndLine  int     // 1-based buffer line, inclusive
	StartCol int     // 1-based column on Line, inclusive
	EndCol   int     // 1-based column on EndLine, exclusive
}

// Listener receives Tidal's OSCContext highlight messages and forwards the
// decoded highlights on a channel.
type Listener struct {
	out    chan Highlight
	server *osc.Server
	addr   string
	mu     sync.Mutex
	conn   net.PacketConn
}

// NewListener creates a listener for Tidal's highlight messages on the given
// address. Call Listen to start receiving.
func NewListener(host string, port int) *Listener {
	l := &Listener{
		out:  make(chan Highlight, chanBuffer),
		addr: net.JoinHostPort(host, strconv.Itoa(port)),
	}

	dispatcher := osc.NewStandardDispatcher()
	err := dispatcher.AddMsgHandler(highlightsAddr, l.handle)
	if err != nil {
		// Unreachable: highlightsAddr contains no pattern characters.
		panic(err)
	}

	l.server = &osc.Server{Dispatcher: dispatcher}
	return l
}

// Highlights returns the channel of decoded highlights. Slow consumers drop
// messages rather than blocking Tidal's sender.
func (l *Listener) Highlights() <-chan Highlight {
	return l.out
}

// Listen starts receiving highlight messages. It blocks until the server
// fails or Close is called, so callers should run it in a goroutine. The
// socket is owned here (not via the go-osc server) so Close cannot race
// with the bind.
func (l *Listener) Listen() error {
	conn, err := net.ListenPacket("udp", l.addr)
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.conn = conn
	l.mu.Unlock()

	err = l.server.Serve(conn)
	if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
		return nil
	}
	return err
}

// Close stops Listen by closing its socket.
func (l *Listener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return nil
	}
	return l.conn.Close()
}

// handle decodes one OSCContext message. Messages for pattern ids outside
// d1..d16 and malformed messages are ignored.
//
// Tidal's column arithmetic yields 0-based buffer indices: deltaContext
// adds the 0-based column of the string literal's opening quote to the
// parser's 1-based in-string column, and the sum is the 0-based buffer
// index of the first character. The Highlight fields carry the project's
// 1-based convention instead, so columns are shifted here.
func (l *Listener) handle(msg *osc.Message) {
	args := msg.Arguments
	if len(args) != numMsgArgs {
		return
	}
	ident, ok := args[0].(string)
	if !ok {
		return
	}
	stream, err := strconv.Atoi(ident)
	if err != nil || stream < tidal.MinStream || stream > tidal.MaxStream {
		return
	}

	cols := make([]int, 4)
	for i, arg := range args[3:] {
		n, ok := asInt(arg)
		if !ok || n < 0 {
			return
		}
		cols[i] = n
	}
	cycle, ok := asFloat(args[2])
	if !ok {
		cycle = 0
	}

	hl := Highlight{
		Stream:   stream,
		Cycle:    cycle,
		StartCol: cols[0] + 1,
		Line:     cols[1],
		EndCol:   cols[2] + 1,
		EndLine:  cols[3],
	}

	select {
	case l.out <- hl:
	default:
	}
}

// asInt accepts the integer types go-osc decodes from int32 arguments.
func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case int:
		return n, true
	}
	return 0, false
}

// asFloat accepts the float types go-osc decodes from float32 arguments.
func asFloat(v any) (float64, bool) {
	switch f := v.(type) {
	case float32:
		return float64(f), true
	case float64:
		return f, true
	}
	return 0, false
}
