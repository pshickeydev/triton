package chbuf

import "testing"

func TestSendDeliversInOrder(t *testing.T) {
	ch := make(chan int, 2)
	Send(ch, 1)
	Send(ch, 2)
	if got := <-ch; got != 1 {
		t.Errorf("got %d, want 1", got)
	}
	if got := <-ch; got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}

func TestSendEvictsOldestWhenFull(t *testing.T) {
	ch := make(chan int, 2)
	Send(ch, 1)
	Send(ch, 2)
	Send(ch, 3)
	if got := <-ch; got != 2 {
		t.Errorf("oldest should be evicted, got %d, want 2", got)
	}
	if got := <-ch; got != 3 {
		t.Errorf("newest should never be dropped, got %d, want 3", got)
	}
}
