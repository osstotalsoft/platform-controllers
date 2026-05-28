package testing

import (
	"testing"
	"time"

	"k8s.io/client-go/util/workqueue"
)

func WaitForQueueLen(t *testing.T, q workqueue.RateLimitingInterface, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		if got := q.Len(); got == want {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	if q.Len() > 0 && want == 0 {
		item, _ := q.Get()
		t.Error("queue should be empty, but contains ", item)
	} else {
		t.Errorf("queue should have %d item(s), but it has %d", want, q.Len())
	}
}

func WaitForQueueEmpty(t *testing.T, q workqueue.RateLimitingInterface) {
	t.Helper()
	WaitForQueueLen(t, q, 0)
}
