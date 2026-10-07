package grpcx

import (
	"errors"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestErrorRoundTrip(t *testing.T) {
	code, reason, msg := Details(Error(codes.PermissionDenied, "USER_BANNED", "Пользователь заблокирован"))
	if code != codes.PermissionDenied || reason != "USER_BANNED" || msg != "Пользователь заблокирован" {
		t.Fatalf("got %v %q %q", code, reason, msg)
	}
	if code, reason, msg = Details(errors.New("plain")); code != codes.Unknown || reason != "" || msg != "" {
		t.Fatalf("plain error: %v %q %q", code, reason, msg)
	}
}

func TestHTTPMappingRoundTrip(t *testing.T) {
	for _, s := range []int{400, 401, 403, 404, 409, 429, 503} {
		if got := HTTPFromCode(CodeFromHTTP(s)); got != s {
			t.Errorf("%d -> %d", s, got)
		}
	}
	if HTTPFromCode(CodeFromHTTP(http.StatusTeapot)) != http.StatusInternalServerError {
		t.Error("unknown status must map to 500")
	}
}
