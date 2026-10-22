package notify

import "testing"

func TestHub(t *testing.T) {
	t.Parallel()
	var h Hub[int]
	ch, stop := h.Subscribe(1)
	other, stopOther := h.Subscribe(2)
	defer stopOther()

	// Changes while the subscriber is busy are merged into one notification.
	h.Notify(1)
	h.Notify(1)
	<-ch
	select {
	case <-ch:
		t.Error("two notifications, want them merged into one")
	default:
	}
	select {
	case <-other:
		t.Error("the subscriber of another key was notified")
	default:
	}

	stop()
	h.Notify(1)
	select {
	case <-ch:
		t.Error("notified after stopping")
	default:
	}
	if _, ok := h.signals[1]; ok {
		t.Error("a key without subscribers is kept")
	}
}
