package dockerproxy

import "testing"

func TestConfigAcceptsBoundedUniqueSelectionOnly(t *testing.T) {
	t.Parallel()
	config, err := NewConfig("/tmp/fake.sock", "127.0.0.1:9123", []string{"api=service-api", "db=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Containers) != 2 || config.Containers[0].Alias != "api" {
		t.Fatalf("config = %#v", config)
	}
	for _, value := range []string{"api=service-api", "bad/name=service", "api=../service", "api=shortid", "api=service-api", "api=service-api", "api=service-api=extra"} {
		if _, err := NewConfig("relative.sock", "127.0.0.1:9123", []string{value}); err == nil {
			t.Errorf("accepted unsafe socket/config %q", value)
		}
	}
	for _, values := range [][]string{{"api=service", "api=other"}, {"one=service", "two=service"}, {}} {
		if _, err := NewConfig("/tmp/fake.sock", "127.0.0.1:9123", values); err == nil {
			t.Errorf("accepted invalid selection %q", values)
		}
	}
}
