package circuit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestBreakerOpenAfterFailures(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	b := New(rdb, "test", Config{FailureThreshold: 3, WindowSeconds: 60, HalfOpenSeconds: 30})
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, _ = b.RecordFailure(ctx, "org", "dest")
	}
	st, _ := b.GetState(ctx, "org", "dest")
	if st.State != StateOpen {
		t.Fatalf("expected open, got %s", st.State)
	}
}
