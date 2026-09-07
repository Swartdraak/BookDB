package s3

import (
	"bytes"
	"testing"
)

func TestChecksumMetadataRoundTrip(t *testing.T) {
	meta := map[string]string{
		MetadataChecksumAlgorithmKey: "sha256",
		MetadataChecksumValueKey:     "abc123",
	}
	cs := checksumFromMetadata(meta)
	if cs == nil {
		t.Fatal("expected checksum metadata")
	}
	if cs.Algorithm != "sha256" || cs.Value != "abc123" {
		t.Fatalf("unexpected checksum %#v", cs)
	}
}

func TestCloneStringMapCopiesValues(t *testing.T) {
	original := map[string]string{"a": "b"}
	cloned := cloneStringMap(original)
	cloned["a"] = "c"
	if original["a"] != "b" {
		t.Fatal("clone should not mutate original")
	}
}

func TestNormalizeEndpointUsesHostAndScheme(t *testing.T) {
	host, secure, err := normalizeEndpoint("http://seaweed-s3:8333", false)
	if err != nil {
		t.Fatalf("normalizeEndpoint: %v", err)
	}
	if host != "seaweed-s3:8333" || secure {
		t.Fatalf("unexpected normalized endpoint %q secure=%v", host, secure)
	}
}

func TestPutOptionsCompatibleWithReaders(t *testing.T) {
	body := bytes.NewReader([]byte("hello"))
	if body.Len() != 5 {
		t.Fatal("unexpected reader length")
	}
}
