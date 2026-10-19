package main

import "testing"

func TestParseConfigRequiresExplicitSocketAndSelection(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"--socket", "/tmp/fake.sock"}, {"--container", "app=service"}, {"--socket", "relative", "--container", "app=service"}, {"--socket", "/tmp/fake.sock", "--container", "app=service", "--listen", "0.0.0.0:9123"}} {
		if _, err := parseConfig(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	config, err := parseConfig([]string{"--socket", "/tmp/fake.sock", "--container", "app=service"})
	if err != nil || config.Listen != "127.0.0.1:9123" {
		t.Fatalf("config = %#v, %v", config, err)
	}
}
