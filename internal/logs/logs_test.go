package logs

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConsumePlainNormalizesBoundsAndSeparatesLines(t *testing.T) {
	var got []struct {
		stream    Stream
		text      string
		truncated bool
	}
	err := ConsumePlain(context.Background(), bytes.NewReader([]byte("ok\x00\n\xff\n"+strings.Repeat("x", MaxTextBytes+20)+"\n")), StreamOutput, func(stream Stream, text string, truncated bool) error {
		got = append(got, struct {
			stream    Stream
			text      string
			truncated bool
		}{stream, text, truncated})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].text != "ok" || got[1].text != "�" || !got[2].truncated || len(got[2].text) != MaxTextBytes {
		t.Fatalf("lines = %#v", got)
	}
}

func TestConsumePlainCountsRawBodyBeforeNormalization(t *testing.T) {
	body := bytes.NewReader(bytes.Repeat([]byte{'\x00'}, MaxInputBytes+1))
	err := ConsumePlain(context.Background(), body, StreamOutput, func(Stream, string, bool) error { return nil })
	if !errors.Is(err, ErrByteLimit) {
		t.Fatalf("error = %v", err)
	}
}

func TestRegistryEmptyCatalogUsesAnEmptyArray(t *testing.T) {
	registry, err := NewRegistry(ModeLive, []Entry{})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := registry.Catalog(context.Background())
	if err != nil || catalog.Sources == nil || len(catalog.Sources) != 0 || ValidateCatalog(catalog) != nil {
		t.Fatalf("catalog = %#v, %v", catalog, err)
	}
}

func TestValidateCatalogRejectsDuplicatesAndOversizeValues(t *testing.T) {
	valid := Catalog{SchemaVersion: SchemaVersion, Mode: ModeLive, Sources: []Source{{ID: "model.alpha", Label: "Alpha", Kind: KindModel}}}
	if err := ValidateCatalog(valid); err != nil {
		t.Fatal(err)
	}
	valid.Sources = append(valid.Sources, valid.Sources[0])
	if err := ValidateCatalog(valid); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate = %v", err)
	}
	if ValidModelReference("proxy") || ValidModelReference(".") || ValidModelReference("..") || ValidModelReference("author/model") || !ValidModelReference("model-1") {
		t.Fatal("model reference policy")
	}
	for _, catalog := range []Catalog{
		{SchemaVersion: SchemaVersion, Mode: ModeDisabled, Sources: []Source{{ID: "model.alpha", Label: "Alpha", Kind: KindModel}}},
		{SchemaVersion: SchemaVersion, Mode: ModeLive, Sources: []Source{{ID: "model.alpha", Label: "bad\nlabel", Kind: KindModel}}},
		{SchemaVersion: SchemaVersion, Mode: ModeLive, Sources: []Source{{ID: "model.alpha", Label: "Alpha", Kind: KindDocker}}},
		{SchemaVersion: SchemaVersion, Mode: ModeLive, Sources: []Source{{ID: "llama.other", Label: "Proxy", Kind: KindLlamaProxy}}},
		{SchemaVersion: SchemaVersion, Mode: ModeDemo, Sources: []Source{{ID: "llama.proxy", Label: "Proxy", Kind: KindLlamaProxy}}},
	} {
		if !errors.Is(ValidateCatalog(catalog), ErrInvalid) {
			t.Errorf("catalog accepted: %#v", catalog)
		}
	}
}

func TestReasonForMapsOnlyContractReasons(t *testing.T) {
	state, reason := ReasonFor(ErrByteLimit)
	if state != StateLimitReached || reason == nil || *reason != ReasonByteLimit {
		t.Fatalf("got %s %#v", state, reason)
	}
	state, reason = ReasonFor(errors.New("private upstream value"))
	if state != StateUnavailable || reason == nil || *reason != ReasonUpstreamUnavailable {
		t.Fatalf("got %s %#v", state, reason)
	}
}
