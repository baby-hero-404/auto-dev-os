package llm

import (
	"errors"
	"testing"
)

type fakeHTTPStatusError struct {
	code int
}

func (e fakeHTTPStatusError) Error() string       { return "http error" }
func (e fakeHTTPStatusError) HTTPStatusCode() int { return e.code }

func TestIsTransientError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"typed 429", fakeHTTPStatusError{429}, true},
		{"typed 402", fakeHTTPStatusError{402}, true},
		{"typed 500", fakeHTTPStatusError{500}, true},
		{"typed 400 (not transient)", fakeHTTPStatusError{400}, false},
		{"status 503 substring", errors.New("provider returned status 503"), true},
		{"rate limit phrase", errors.New("you have hit the rate limit"), true},
		{"quota phrase", errors.New("provider quota exceeded"), true},
		{"temporarily unavailable", errors.New("service temporarily unavailable"), true},
		{"high demand", errors.New("model is at high demand right now"), true},
		{"timeout", errors.New("context deadline exceeded (Client.Timeout exceeded)"), true},
		{"connection refused", errors.New("dial tcp: connection refused"), true},
		{"eof", errors.New("unexpected EOF"), true},
		{"unrelated error", errors.New("invalid api key"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTransientError(tc.err); got != tc.want {
				t.Errorf("IsTransientError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
