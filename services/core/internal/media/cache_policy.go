package media

import (
	"errors"
)

const (
	CacheAutomatic = "automatic"
	CacheManual    = "manual"
	CacheRAM       = "ram"
	CacheDisk      = "disk"
)

var ErrInvalidCachePolicy = errors.New("invalid media cache policy")
var ErrInsufficientCache = errors.New("not enough safe cache capacity")

// CachePolicy is the admin-controlled, DB-persisted policy. In automatic mode,
// storage and limits are calculated from actual free capacity on the worker.
// Manual limits are hard ceilings, not a reservation of filesystem space.
type CachePolicy struct {
	Mode                string `json:"mode"`
	Storage             string `json:"storage"`
	MaxMiB              int    `json:"maxMiB"`
	MaxConcurrent       int    `json:"maxConcurrent"`
	SegmentSeconds      int    `json:"segmentSeconds"`
	IdleSeconds         int    `json:"idleSeconds"`
	CleanupGraceSeconds int    `json:"cleanupGraceSeconds"`
}

type CacheCapacity struct {
	RAMFreeMiB  int
	DiskFreeMiB int
	CPUCount    int
}

type EffectiveCachePolicy struct {
	CachePolicy
	Reason    string `json:"reason"`
	Effective bool   `json:"effective"`
}

func DefaultCachePolicy() CachePolicy {
	return CachePolicy{
		Mode:                CacheAutomatic,
		Storage:             CacheRAM,
		MaxMiB:              128,
		MaxConcurrent:       1,
		SegmentSeconds:      3,
		IdleSeconds:         90,
		CleanupGraceSeconds: 15,
	}
}

func (p CachePolicy) Validate() error {
	if (p.Mode != CacheAutomatic && p.Mode != CacheManual) ||
		(p.Storage != CacheRAM && p.Storage != CacheDisk) ||
		p.MaxMiB < 64 || p.MaxMiB > 4096 ||
		p.MaxConcurrent < 1 || p.MaxConcurrent > 4 ||
		p.SegmentSeconds < 2 || p.SegmentSeconds > 10 ||
		p.IdleSeconds < 30 || p.IdleSeconds > 600 ||
		p.CleanupGraceSeconds < 0 || p.CleanupGraceSeconds > 180 {
		return ErrInvalidCachePolicy
	}
	return nil
}

// ResolveCachePolicy uses conservative worker-reported free RAM/disk capacity.
// Automatic mode uses RAM preferentially; it never silently spills onto SSD.
// When RAM cannot hold even the 64 MiB minimum, report unavailable instead of
// changing storage unexpectedly. Operators may explicitly choose disk in manual.
func ResolveCachePolicy(p CachePolicy, c CacheCapacity) (EffectiveCachePolicy, error) {
	if err := p.Validate(); err != nil {
		return EffectiveCachePolicy{}, err
	}
	e := EffectiveCachePolicy{CachePolicy: p, Reason: "Configuração manual", Effective: true}
	if p.Mode == CacheAutomatic {
		e.Storage = CacheRAM
		// Leave the greater of 256 MiB or 75% of current free memory alone.
		free := max(0, c.RAMFreeMiB)
		available := min(free/4, max(0, free-256))
		if available < 64 {
			return EffectiveCachePolicy{}, ErrInsufficientCache
		}
		e.MaxMiB = min(128, available)
		e.MaxConcurrent = 1
		e.SegmentSeconds = 3
		e.IdleSeconds = 90
		e.CleanupGraceSeconds = 15
		e.Reason = "RAM automática; sem gravação em disco"
	} else {
		free := c.DiskFreeMiB
		if p.Storage == CacheRAM {
			free = c.RAMFreeMiB
		}
		// Reserve room for the OS and other services instead of trusting maxMiB.
		reserve := 128
		if p.Storage == CacheDisk {
			reserve = 1024
		}
		if free-p.MaxMiB < reserve {
			return EffectiveCachePolicy{}, ErrInsufficientCache
		}
	}
	e.Mode = p.Mode
	return e, nil
}
