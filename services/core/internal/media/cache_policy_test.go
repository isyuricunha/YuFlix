package media

import (
	"errors"
	"testing"
)

func TestCacheDefaultsAndAutomaticSelection(t *testing.T) {
	defaults := DefaultCachePolicy()
	if defaults.Mode != CacheAutomatic || defaults.Storage != CacheRAM ||
		defaults.MaxMiB != 128 || defaults.MaxConcurrent != 1 ||
		defaults.IdleSeconds != 90 || defaults.CleanupGraceSeconds != 15 {
		t.Fatalf("unsafe defaults: %+v", defaults)
	}
	if err := defaults.Validate(); err != nil {
		t.Fatal(err)
	}
	result, err := ResolveCachePolicy(defaults, CacheCapacity{RAMFreeMiB: 1200, DiskFreeMiB: 120000})
	if err != nil || result.Storage != CacheRAM || result.MaxMiB != 128 || result.MaxConcurrent != 1 {
		t.Fatalf("unsafe automatic policy %+v (%v)", result, err)
	}
	if _, err := ResolveCachePolicy(defaults, CacheCapacity{RAMFreeMiB: 300, DiskFreeMiB: 100000}); !errors.Is(err, ErrInsufficientCache) {
		t.Fatalf("RAM exhaustion must fail closed, got %v", err)
	}
}

func TestCacheManualRulesAndExhaustion(t *testing.T) {
	policy := DefaultCachePolicy()
	policy.Mode = CacheManual
	policy.Storage = CacheDisk
	policy.MaxMiB = 512
	policy.MaxConcurrent = 2
	policy.IdleSeconds = 60
	policy.CleanupGraceSeconds = 0
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveCachePolicy(policy, CacheCapacity{RAMFreeMiB: 256, DiskFreeMiB: 2048})
	if err != nil || got.Storage != CacheDisk || got.MaxMiB != 512 || got.MaxConcurrent != 2 {
		t.Fatalf("manual storage ignored %+v (%v)", got, err)
	}
	if _, err := ResolveCachePolicy(policy, CacheCapacity{RAMFreeMiB: 10000, DiskFreeMiB: 800}); !errors.Is(err, ErrInsufficientCache) {
		t.Fatalf("disk minimum reserve bypassed: %v", err)
	}
	policy.MaxMiB = 6000
	if err := policy.Validate(); !errors.Is(err, ErrInvalidCachePolicy) {
		t.Fatal("oversized cache accepted", err)
	}
	policy = DefaultCachePolicy()
	policy.IdleSeconds = 0
	if err := policy.Validate(); !errors.Is(err, ErrInvalidCachePolicy) {
		t.Fatal("unsafe idle timeout accepted", err)
	}
}
