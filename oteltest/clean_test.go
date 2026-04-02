package oteltest

import (
	"testing"
)

func TestCleanErrorMessage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain message unchanged",
			input: "something went wrong",
			want:  "something went wrong",
		},
		{
			name:  "whitespace trimmed",
			input: "  something went wrong  ",
			want:  "something went wrong",
		},
		{
			name:  "testify single-line error",
			input: "\n\tError Trace:\t/app/test.go:41\n\tError:      \tExpected nil, but got error\n\tTest:       \tTestFoo\n",
			want:  "Expected nil, but got error",
		},
		{
			name: "testify multi-line error with long trace",
			input: "\n\tError Trace:\t/app/core/integration/module_iface_test.go:41\n" +
				"\t            \t\t\t\t/go/pkg/mod/github.com/dagger/testctx@v0.1.2/testctx.go:296\n" +
				"\t            \t\t\t\t/go/pkg/mod/github.com/dagger/testctx/oteltest@v0.1.2/log.go:37\n" +
				"\t            \t\t\t\t/go/pkg/mod/github.com/dagger/testctx/oteltest@v0.1.2/trace.go:94\n" +
				"\t            \t\t\t\t/go/pkg/mod/github.com/dagger/testctx@v0.1.2/middleware.go:25\n" +
				"\t            \t\t\t\t/go/pkg/mod/github.com/dagger/testctx@v0.1.2/testctx.go:150\n" +
				"\tError:      \tReceived unexpected error:\n" +
				"\t            \texit code: 1 [traceparent:0b47577d7593bdb744a5bbe21b4d7479-30b0c866d891a343]\n" +
				"\tTest:       \tTestInterface/TestIfaceBasic/go\n",
			want: "Received unexpected error:\nexit code: 1 [traceparent:0b47577d7593bdb744a5bbe21b4d7479-30b0c866d891a343]",
		},
		{
			name: "testify with messages section",
			input: "\n\tError Trace:\t/app/test.go:41\n" +
				"\tError:      \tReceived unexpected error:\n" +
				"\t            \tsomething failed\n" +
				"\tMessages:   \tadditional context\n" +
				"\tTest:       \tTestFoo\n",
			want: "Received unexpected error:\nsomething failed\nadditional context",
		},
		{
			name:  "message with tab-Error but not testify format",
			input: "some\tError: happened",
			want:  "some\tError: happened",
		},
		{
			name:  "empty message",
			input: "",
			want:  "",
		},
		{
			name:  "newline-only message",
			input: "\n\n",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanErrorMessage(tc.input)
			if got != tc.want {
				t.Errorf("cleanErrorMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}
