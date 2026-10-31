package media

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var ErrWorkerBusy = errors.New("media worker concurrency limit reached")
var ErrWorkerQuota = errors.New("media worker cache quota reached")
var ErrInvalidWorkerRoot = errors.New("unsafe media worker directory")

// LocalHLSWorker runs FFmpeg only against approved, existing local media.
// It has no HTTP handlers, external network access, or arbitrary filesystem
// input. A separate unprivileged worker container is still required before
// exposing this engine to real clients.
type LocalHLSWorker struct {
	mu        sync.Mutex
	inputRoot string
	ramRoot   string
	diskRoot  string
	binary    string
	jobs      map[string]*LocalHLSJob
	shutdown  bool
}

type LocalHLSJob struct {
	ID           string
	Directory    string
	PlaylistPath string
	Storage      string
	CreatedAt    time.Time

	mu           sync.Mutex
	cancel       context.CancelFunc
	done         chan struct{}
	err          error
	lastAccess   time.Time
	closed       bool
	cleanupAfter time.Duration
}

func (job *LocalHLSJob) Err() error {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.err
}
func (job *LocalHLSJob) Touch() {
	job.mu.Lock()
	defer job.mu.Unlock()
	if !job.closed {
		job.lastAccess = time.Now()
	}
}
func (job *LocalHLSJob) Done() <-chan struct{} { return job.done }

func validWorkerRoot(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) == string(filepath.Separator) {
		return "", ErrInvalidWorkerRoot
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0002 != 0 {
		return "", ErrInvalidWorkerRoot
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || real != filepath.Clean(path) {
		return "", ErrInvalidWorkerRoot
	}
	return real, nil
}
func NewLocalHLSWorker(binary, inputRoot, ramRoot, diskRoot string) (*LocalHLSWorker, error) {
	if binary == "" || strings.Contains(binary, "://") {
		return nil, ErrInvalidWorkerRoot
	}
	var roots [3]string
	for i, s := range []string{inputRoot, ramRoot, diskRoot} {
		p, err := validWorkerRoot(s)
		if err != nil {
			return nil, err
		}
		roots[i] = p
	}
	if roots[0] == roots[1] || roots[0] == roots[2] || roots[1] == roots[2] {
		return nil, ErrInvalidWorkerRoot
	}
	// The directories must be separate paths (no overlapping parents).
	for i := range roots {
		for j := range roots {
			if i != j && strings.HasPrefix(roots[i]+string(os.PathSeparator), roots[j]+string(os.PathSeparator)) {
				return nil, ErrInvalidWorkerRoot
			}
		}
	}
	return &LocalHLSWorker{binary: binary, inputRoot: roots[0], ramRoot: roots[1], diskRoot: roots[2], jobs: map[string]*LocalHLSJob{}}, nil
}
func fsCapacity(root string) (int, error) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(root, &stats); err != nil {
		return 0, err
	}
	free := (uint64(stats.Bavail) * uint64(stats.Bsize)) / (1024 * 1024)
	if free > 1<<31 {
		free = 1 << 31
	}
	return int(free), nil
}
func dirBytes(root string) int64 {
	var size int64
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.Type().IsRegular() {
			if info, statErr := entry.Info(); statErr == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}
func (worker *LocalHLSWorker) capacity() (CacheCapacity, error) {
	ram, err := fsCapacity(worker.ramRoot)
	if err != nil {
		return CacheCapacity{}, err
	}
	disk, err := fsCapacity(worker.diskRoot)
	if err != nil {
		return CacheCapacity{}, err
	}
	return CacheCapacity{RAMFreeMiB: ram, DiskFreeMiB: disk}, nil
}
func (worker *LocalHLSWorker) cacheRoot(mode string) string {
	if mode == CacheDisk {
		return worker.diskRoot
	}
	return worker.ramRoot
}

func (worker *LocalHLSWorker) Start(ctx context.Context, input string, videoIndex, audioIndex int,
	plan Plan, transfer string, seek float64, policy CachePolicy) (*LocalHLSJob, error) {
	capacity, err := worker.capacity()
	if err != nil {
		return nil, err
	}
	effective, err := ResolveCachePolicy(policy, capacity)
	if err != nil {
		return nil, err
	}
	worker.mu.Lock()
	if worker.shutdown {
		worker.mu.Unlock()
		return nil, context.Canceled
	}
	if len(worker.jobs) >= effective.MaxConcurrent {
		worker.mu.Unlock()
		return nil, ErrWorkerBusy
	}
	root := worker.cacheRoot(effective.Storage)
	total := int64(0)
	for _, job := range worker.jobs {
		if job.Storage == effective.Storage {
			total += dirBytes(job.Directory)
		}
	}
	if total >= int64(effective.MaxMiB)*1024*1024 {
		worker.mu.Unlock()
		return nil, ErrWorkerQuota
	}
	worker.mu.Unlock()
	// Do not pass remote addresses, provider tokens or unapproved file paths.
	approved, err := filepath.EvalSymlinks(input)
	if err != nil {
		return nil, ErrUnsafeLocalMedia
	}
	relative, err := filepath.Rel(worker.inputRoot, approved)
	if err != nil || relative == "." || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, ErrUnsafeLocalMedia
	}
	info, err := os.Stat(approved)
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrUnsafeLocalMedia
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(nonce[:])
	dir, err := os.MkdirTemp(root, "job-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	spec := HLSSpec{
		InputPath: approved, PlaylistPath: filepath.Join(dir, "index.m3u8"),
		SegmentTemplate: filepath.Join(dir, "chunk-%05d.ts"),
		VideoIndex:      videoIndex, AudioIndex: audioIndex, SeekSeconds: seek, SegmentSeconds: effective.SegmentSeconds,
		HDRTransfer: transfer, Plan: plan,
	}
	args, err := HLSArguments(spec)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, worker.binary, args...)
	// FFmpeg must not inherit sensitive application environments or credentials.
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/nonexistent", "LC_ALL=C"}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		cancel()
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("worker could not start: %w", err)
	}
	job := &LocalHLSJob{
		ID: id, Directory: dir, PlaylistPath: spec.PlaylistPath,
		Storage: effective.Storage, CreatedAt: time.Now(), lastAccess: time.Now(),
		cleanupAfter: time.Duration(effective.CleanupGraceSeconds) * time.Second,
		cancel:       cancel, done: make(chan struct{}),
	}
	worker.mu.Lock()
	if worker.shutdown || len(worker.jobs) >= effective.MaxConcurrent {
		worker.mu.Unlock()
		cancel()
		_ = cmd.Wait()
		_ = os.RemoveAll(dir)
		return nil, ErrWorkerBusy
	}
	worker.jobs[id] = job
	worker.mu.Unlock()

	go worker.watch(job, cmd, effective)
	return job, nil
}

func (worker *LocalHLSWorker) watch(job *LocalHLSJob, cmd *exec.Cmd, policy EffectiveCachePolicy) {
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	timer := time.NewTicker(250 * time.Millisecond)
	defer timer.Stop()
	var runErr error
	for {
		select {
		case runErr = <-wait:
			goto finished
		case <-timer.C:
			job.mu.Lock()
			idle := time.Since(job.lastAccess) > time.Duration(policy.IdleSeconds)*time.Second
			job.mu.Unlock()
			if idle || dirBytes(worker.cacheRoot(policy.Storage)) > int64(policy.MaxMiB)*1024*1024 {
				job.cancel()
			}
		}
	}
finished:
	job.mu.Lock()
	job.err = runErr
	job.closed = true
	job.mu.Unlock()
	job.cancel()
	close(job.done)
	time.AfterFunc(job.cleanupAfter, func() {
		worker.mu.Lock()
		defer worker.mu.Unlock()
		if item, ok := worker.jobs[job.ID]; ok && item == job {
			delete(worker.jobs, job.ID)
			_ = os.RemoveAll(job.Directory)
		}
	})
}

func (worker *LocalHLSWorker) Stop(id string) bool {
	worker.mu.Lock()
	job, ok := worker.jobs[id]
	worker.mu.Unlock()
	if ok {
		job.cancel()
	}
	return ok
}

func (worker *LocalHLSWorker) Shutdown() {
	worker.mu.Lock()
	worker.shutdown = true
	jobs := make([]*LocalHLSJob, 0, len(worker.jobs))
	for _, job := range worker.jobs {
		jobs = append(jobs, job)
	}
	worker.mu.Unlock()
	for _, job := range jobs {
		job.cancel()
	}
	for _, job := range jobs {
		<-job.Done()
		worker.mu.Lock()
		delete(worker.jobs, job.ID)
		_ = os.RemoveAll(job.Directory)
		worker.mu.Unlock()
	}
}

// RemoveOrphanedCache only removes named job-* folders inside explicitly
// reserved media cache roots. It must be run before accepting new sessions,
// while no older worker process owns those directories.
func (worker *LocalHLSWorker) RemoveOrphanedCache() error {
	worker.mu.Lock()
	defer worker.mu.Unlock()
	if len(worker.jobs) > 0 {
		return ErrWorkerBusy
	}
	for _, root := range []string{worker.ramRoot, worker.diskRoot} {
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), "job-") {
				continue
			}
			p := filepath.Join(root, entry.Name())
			if err := os.RemoveAll(p); err != nil {
				return err
			}
		}
	}
	return nil
}
