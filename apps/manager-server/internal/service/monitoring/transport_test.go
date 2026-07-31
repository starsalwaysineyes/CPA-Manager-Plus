package monitoring

import "testing"

func TestEventTransport(t *testing.T) {
	tests := []struct {
		executorType string
		want         string
	}{
		{executorType: "CodexExecutor", want: "http"},
		{executorType: "OpenAICompatExecutor", want: "http"},
		{executorType: "", want: "http"},
		{executorType: "CodexWebsocketsExecutor", want: "websocket"},
		{executorType: "websocket", want: "websocket"},
	}

	for _, test := range tests {
		if got := eventTransport(test.executorType); got != test.want {
			t.Fatalf("eventTransport(%q) = %q, want %q", test.executorType, got, test.want)
		}
	}
}
