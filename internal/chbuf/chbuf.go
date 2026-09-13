// Package chbuf provides non-blocking delivery to bounded channels.
package chbuf

// Send delivers v on ch without blocking. When ch is full, the oldest
// buffered value is evicted first, so the most recent value (typically
// the relevant errors) is never the one dropped.
func Send[T any](ch chan T, v T) {
	select {
	case ch <- v:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- v:
	default:
	}
}
