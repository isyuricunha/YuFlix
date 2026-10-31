package httpapi

import (
	"context"
	"errors"
	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"testing"
)

func TestSafeProviderReasonOnlyReturnsSanitizedCategories(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{&catalog.UpstreamHTTPError{StatusCode: 403}, "access_denied"},
		{&catalog.UpstreamHTTPError{StatusCode: 429}, "rate_limited"},
		{&catalog.UpstreamHTTPError{StatusCode: 502}, "upstream_unavailable"},
		{context.DeadlineExceeded, "timeout"},
		{catalog.ErrUnsafeURL, "unsafe_address"},
		{catalog.ErrInvalid, "invalid_response"},
		{errors.New("sensitive private upstream URL"), "connection_failed"},
	}
	for _, c := range cases {
		if got := safeProviderReason(c.err); got != c.want {
			t.Fatalf("expected %s, got %s", c.want, got)
		}
	}
}
