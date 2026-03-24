package main

import (
	"errors"
	"testing"
	"time"
)

func TestCertRetryLoop_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	createFn := func() (error, error, error, error, error) {
		calls++
		return nil, nil, nil, nil, nil
	}

	result := certRetryLoop(3, time.Millisecond, createFn)
	if !result {
		t.Fatal("expected httpsEnabled=true when cert creation succeeds on first attempt")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestCertRetryLoop_SuccessAfterRetry(t *testing.T) {
	calls := 0
	createFn := func() (error, error, error, error, error) {
		calls++
		if calls < 3 {
			return errors.New("ca error"), nil, nil, nil, nil
		}
		return nil, nil, nil, nil, nil
	}

	result := certRetryLoop(3, time.Millisecond, createFn)
	if !result {
		t.Fatal("expected httpsEnabled=true when cert creation succeeds on third attempt")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestCertRetryLoop_AllRetriesFail(t *testing.T) {
	calls := 0
	createFn := func() (error, error, error, error, error) {
		calls++
		return errors.New("ca"), errors.New("ser"), errors.New("cli"), nil, nil
	}

	result := certRetryLoop(3, time.Millisecond, createFn)
	if result {
		t.Fatal("expected httpsEnabled=false when all retries fail")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestCertRetryLoop_PartialErrors(t *testing.T) {
	calls := 0
	createFn := func() (error, error, error, error, error) {
		calls++
		// Only serverSecretErr fails on every attempt
		return nil, nil, nil, errors.New("secret error"), nil
	}

	result := certRetryLoop(2, time.Millisecond, createFn)
	if result {
		t.Fatal("expected httpsEnabled=false when partial errors persist")
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestCertRetryLoop_SingleRetry(t *testing.T) {
	calls := 0
	createFn := func() (error, error, error, error, error) {
		calls++
		return errors.New("fail"), nil, nil, nil, nil
	}

	result := certRetryLoop(1, time.Millisecond, createFn)
	if result {
		t.Fatal("expected httpsEnabled=false with single retry and failure")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestCertRetryLoop_ExponentialBackoff(t *testing.T) {
	calls := 0
	timestamps := make([]time.Time, 0)
	createFn := func() (error, error, error, error, error) {
		calls++
		timestamps = append(timestamps, time.Now())
		if calls < 3 {
			return errors.New("fail"), nil, nil, nil, nil
		}
		return nil, nil, nil, nil, nil
	}

	initialDelay := 50 * time.Millisecond
	result := certRetryLoop(3, initialDelay, createFn)
	if !result {
		t.Fatal("expected success on third attempt")
	}

	if len(timestamps) != 3 {
		t.Fatalf("expected 3 timestamps, got %d", len(timestamps))
	}

	// First retry delay should be >= initialDelay (50ms)
	firstGap := timestamps[1].Sub(timestamps[0])
	if firstGap < initialDelay/2 {
		t.Errorf("first retry gap %v is too short (expected ~%v)", firstGap, initialDelay)
	}

	// Second retry delay should be >= 2*initialDelay (100ms)
	secondGap := timestamps[2].Sub(timestamps[1])
	if secondGap < initialDelay {
		t.Errorf("second retry gap %v is too short (expected ~%v)", secondGap, 2*initialDelay)
	}
}
