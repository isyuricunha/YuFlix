package httpapi

import (
	"context"
	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
	"time"
)

type autoResult struct {
	report catalog.StreamReport
	remote bool
	err    error
}

func (a *api) discoverAll(ctx context.Context, typ, id string, sources []db.StreamAddon) []<-chan autoResult {
	output := make([]<-chan autoResult, len(sources))
	slots := make(chan struct{}, 3)
	for i, source := range sources {
		ch := make(chan autoResult, 1)
		output[i] = ch
		go func(s db.StreamAddon, out chan autoResult) {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-slots }()
			sourceCtx, cancel := context.WithTimeout(ctx, time.Duration(s.TimeoutMs)*time.Millisecond)
			defer cancel()
			report, remote, err := a.catalogClient.RouteStreams(sourceCtx, s.ManifestURL, typ, id, s.NetworkRoute)
			select {
			case out <- autoResult{report, remote, err}:
			case <-ctx.Done():
			}
		}(source, ch)
	}
	return output
}
