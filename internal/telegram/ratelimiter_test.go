package telegram

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewAPIRateLimiter(t *testing.T) {
	rl := NewAPIRateLimiter()

	if rl == nil {
		t.Fatal("NewAPIRateLimiter returned nil")
	}

	if rl.maxTokens != 30.0 {
		t.Errorf("Expected maxTokens to be 30.0, got %.2f", rl.maxTokens)
	}

	if rl.refillRate != 30.0 {
		t.Errorf("Expected refillRate to be 30.0, got %.2f", rl.refillRate)
	}

	if rl.tokens != 30.0 {
		t.Errorf("Expected initial tokens to be 30.0, got %.2f", rl.tokens)
	}
}

func TestAPIRateLimiter_Wait_AllowsImmediateAccess(t *testing.T) {
	rl := NewAPIRateLimiter()
	ctx := context.Background()

	// First request should succeed immediately
	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Should complete almost instantly (within 10ms)
	if elapsed > 10*time.Millisecond {
		t.Errorf("Wait took too long: %v", elapsed)
	}
}

func TestAPIRateLimiter_Wait_RespectsCancellation(t *testing.T) {
	rl := NewAPIRateLimiter()

	// Exhaust all tokens
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Failed to exhaust tokens at iteration %d: %v", i, err)
		}
	}

	// Create a context that's already cancelled
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should fail immediately with context.Canceled error
	err := rl.Wait(cancelledCtx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestAPIRateLimiter_Wait_RespectsCancellationDuringWait(t *testing.T) {
	rl := NewAPIRateLimiter()
	ctx := context.Background()

	// Exhaust all tokens
	for i := 0; i < 30; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Failed to exhaust tokens: %v", err)
		}
	}

	// Create a context that will be cancelled during wait
	// Use a very short timeout (10ms) to ensure it times out before refill (~33ms per token)
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// This should fail with deadline exceeded
	start := time.Now()
	err := rl.Wait(timeoutCtx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}

	// Should have waited approximately the timeout duration (allow for timing variance)
	if elapsed < 5*time.Millisecond || elapsed > 50*time.Millisecond {
		t.Logf("Wait duration: %v (expected 5-50ms)", elapsed)
	}
}

func TestAPIRateLimiter_Wait_EnforcesRateLimit(t *testing.T) {
	rl := NewAPIRateLimiter()
	ctx := context.Background()

	// Make 30 requests (full bucket)
	for i := 0; i < 30; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Failed request %d: %v", i, err)
		}
	}

	// The 31st request should have to wait
	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// Should have waited at least 20ms (tokens refill at 30/sec = ~33ms per token)
	// We allow some variance for timing precision
	if elapsed < 20*time.Millisecond {
		t.Errorf("Expected to wait, but completed in %v", elapsed)
	}
}

func TestAPIRateLimiter_Refill(t *testing.T) {
	rl := NewAPIRateLimiter()
	ctx := context.Background()

	// Exhaust all tokens
	for i := 0; i < 30; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Failed to exhaust tokens: %v", err)
		}
	}

	// Wait for 1 second to allow refill
	time.Sleep(1 * time.Second)

	// We should be able to make 30 more requests without blocking significantly
	start := time.Now()
	for i := 0; i < 30; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Failed request %d after refill: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// All 30 requests should complete quickly (within 100ms)
	if elapsed > 100*time.Millisecond {
		t.Errorf("Requests after refill took too long: %v", elapsed)
	}
}

func TestAPIRateLimiter_GetStats(t *testing.T) {
	rl := NewAPIRateLimiter()
	ctx := context.Background()

	// Check initial stats
	available, maxTokens := rl.GetStats()
	if available != 30.0 {
		t.Errorf("Expected 30.0 available tokens, got %.2f", available)
	}
	if maxTokens != 30.0 {
		t.Errorf("Expected 30.0 max tokens, got %.2f", maxTokens)
	}

	// Use some tokens
	for i := 0; i < 10; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Failed to use token: %v", err)
		}
	}

	// Check stats after using tokens
	available, maxTokens = rl.GetStats()
	if available < 19.0 || available > 21.0 {
		t.Errorf("Expected ~20.0 available tokens, got %.2f", available)
	}
	if maxTokens != 30.0 {
		t.Errorf("Expected max to remain 30.0, got %.2f", maxTokens)
	}
}

func TestAPIRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewAPIRateLimiter()
	ctx := context.Background()

	const numGoroutines = 50
	errors := make(chan error, numGoroutines)
	start := time.Now()

	// Launch multiple goroutines trying to acquire tokens concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			errors <- rl.Wait(ctx)
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Goroutine %d failed: %v", i, err)
		}
	}

	elapsed := time.Since(start)

	// With 30 tokens/second and 50 requests, we expect:
	// - First 30 requests: immediate
	// - Next 20 requests: need to wait for refill
	// - Minimum time: 20 requests / 30 tokens per second ≈ 667ms
	// We allow some tolerance for timing precision
	expectedMinDuration := 600 * time.Millisecond
	if elapsed < expectedMinDuration {
		t.Errorf("Expected at least %v for 50 concurrent requests, got %v", expectedMinDuration, elapsed)
	}
}

func TestAPIRateLimiter_SustainedLoad(t *testing.T) {
	rl := NewAPIRateLimiter()
	ctx := context.Background()

	// Make 60 requests over 2+ seconds
	// This should average to 30 requests per second
	start := time.Now()
	for i := 0; i < 60; i++ {
		if err := rl.Wait(ctx); err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// With 30 tokens/second rate:
	// - First 30 requests: immediate (use initial bucket)
	// - Next 30 requests: need 1 second to refill
	// - Total expected time: ~1 second minimum
	// We allow up to 1.5 seconds for timing variance
	if elapsed < 950*time.Millisecond {
		t.Errorf("Requests completed too quickly: %v (expected >= 950ms)", elapsed)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("Requests took too long: %v (expected <= 1.5s)", elapsed)
	}
}

func BenchmarkAPIRateLimiter_Wait(b *testing.B) {
	rl := NewAPIRateLimiter()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := rl.Wait(ctx); err != nil {
			b.Fatalf("Wait failed: %v", err)
		}
	}
}

func BenchmarkAPIRateLimiter_GetStats(b *testing.B) {
	rl := NewAPIRateLimiter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rl.GetStats()
	}
}
