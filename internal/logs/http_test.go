package logs

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestOpenHTTPBoundsBlockedDialWithoutLeakingOpeningTimer(t *testing.T) {
	original := openingTimeout
	openingTimeout = 20 * time.Millisecond
	t.Cleanup(func() { openingTimeout = original })
	client := newHTTPClient(func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	request, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:1/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, finish, err := OpenHTTP(context.Background(), client, request)
	finish()
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("blocked opening = %v after %s", err, time.Since(started))
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("opening error = %v", err)
	}
}
