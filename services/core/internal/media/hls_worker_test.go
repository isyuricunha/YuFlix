package media

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func testWorker(t *testing.T) (*LocalHLSWorker, string) {
	t.Helper()
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is optional on development environments")
	}
	base := t.TempDir()
	paths := []string{filepath.Join(base, "input"), filepath.Join(base, "ram"), filepath.Join(base, "disk")}
	for _, p := range paths {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	w, err := NewLocalHLSWorker(binary, paths[0], paths[1], paths[2])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Shutdown)
	return w, paths[0]
}

func syntheticSource(t *testing.T, root string) string {
	t.Helper()
	video := filepath.Join(root, "source.mkv")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=blue:s=320x180:r=12",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-t", "3", "-threads", "1", "-c:v", "mpeg4", "-c:a", "aac", "-y", video)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture ffmpeg error: %v (%s)", err, output)
	}
	return video
}

func TestLocalWorkerGeneratesHLSAndDeletesFinishedSession(t *testing.T) {
	w, root := testWorker(t)
	video := syntheticSource(t, root)
	policy := DefaultCachePolicy()
	policy.Mode = CacheManual
	policy.Storage = CacheRAM
	policy.MaxMiB = 64
	policy.CleanupGraceSeconds = 0
	job, err := w.Start(context.Background(), video, 0, 1,
		Plan{Mode: TranscodeVideo, CopyVideo: false, CopyAudio: true}, "", 0, policy)
	if errors.Is(err, ErrInsufficientCache) {
		t.Skip("not enough available test RAM")
	}
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-job.Done():
	case <-time.After(9 * time.Second):
		t.Fatal("FFmpeg job exceeded deadline")
	}
	if err := job.Err(); err != nil {
		t.Fatalf("HLS job failed: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(job.Directory); os.IsNotExist(err) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("completed stream cache was not removed")
}

func TestLocalWorkerExplicitCancellationRemovesTemporaryFiles(t *testing.T) {
	w, root := testWorker(t)
	video := syntheticSource(t, root)
	policy := DefaultCachePolicy()
	policy.Mode = CacheManual
	policy.Storage = CacheRAM
	policy.MaxMiB = 64
	policy.CleanupGraceSeconds = 0
	job, err := w.Start(context.Background(), video, 0, 1,
		Plan{Mode: TranscodeVideo, CopyVideo: false, CopyAudio: true}, "", 0, policy)
	if errors.Is(err, ErrInsufficientCache) {
		t.Skip("insufficient RAM")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !w.Stop(job.ID) {
		t.Fatal("active job not cancelled")
	}
	select {
	case <-job.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not stop FFmpeg")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(job.Directory); os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cancelled media cache not deleted")
}

func TestLocalWorkerRejectsOutsideAndCleansOrphans(t *testing.T) {
	w, _ := testWorker(t)
	outside := filepath.Join(t.TempDir(), "out.mp4")
	if err := os.WriteFile(outside, []byte("untrusted"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := w.Start(context.Background(), outside, 0, -1, Plan{Mode: Remux, CopyVideo: true, CopyAudio: true}, "", 0, DefaultCachePolicy())
	if !errors.Is(err, ErrUnsafeLocalMedia) && !errors.Is(err, ErrInsufficientCache) {
		t.Fatalf("outside file allowed: %v", err)
	}
	orphan := filepath.Join(w.ramRoot, "job-abandoned")
	if err := os.Mkdir(orphan, 0700); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(w.ramRoot, "keep-user-data")
	if err := os.Mkdir(protected, 0700); err != nil {
		t.Fatal(err)
	}
	if err := w.RemoveOrphanedCache(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("orphan remained")
	}
	if _, err := os.Stat(protected); err != nil {
		t.Fatal("unowned directory removed")
	}
}

func TestWorkerInvalidRoots(t *testing.T) {
	root := t.TempDir()
	if _, err := NewLocalHLSWorker("ffmpeg", root, root, root); !errors.Is(err, ErrInvalidWorkerRoot) {
		t.Fatalf("duplicate roots accepted %v", err)
	}
	if _, err := NewLocalHLSWorker("ffmpeg", "/", "/tmp", "/var/tmp"); !errors.Is(err, ErrInvalidWorkerRoot) {
		t.Fatalf("filesystem root accepted %v", err)
	}
}
