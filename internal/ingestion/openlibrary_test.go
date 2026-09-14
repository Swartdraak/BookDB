package ingestion

import (
	"strings"
	"testing"
)

func TestParseOpenLibraryLineJSONL(t *testing.T) {
	line := `{"key":"/works/OL1W","title":"Dune","languages":[{"key":"/languages/en"}]}`
	rec, err := ParseOpenLibraryLine(line)
	if err != nil {
		t.Fatalf("ParseOpenLibraryLine() error = %v", err)
	}
	if rec == nil {
		t.Fatal("ParseOpenLibraryLine() returned nil record")
	}
	if rec.SourceType != "work" {
		t.Fatalf("SourceType = %q, want work", rec.SourceType)
	}
	if rec.SourceKey != "OL1W" {
		t.Fatalf("SourceKey = %q, want OL1W", rec.SourceKey)
	}
	if rec.Title != "Dune" {
		t.Fatalf("Title = %q, want Dune", rec.Title)
	}
}

func TestParseOpenLibraryLineCompleteDumpTSV(t *testing.T) {
	line := "/type/author\t/authors/OL5345273A\t1\t2008-10-05T09:14:51.676778\t{\"name\":\"Aksakov, S. T.\",\"key\":\"/authors/OL5345273A\",\"type\":{\"key\":\"/type/author\"},\"revision\":1}"
	rec, err := ParseOpenLibraryLine(line)
	if err != nil {
		t.Fatalf("ParseOpenLibraryLine() error = %v", err)
	}
	if rec == nil {
		t.Fatal("ParseOpenLibraryLine() returned nil record")
	}
	if rec.SourceType != "author" {
		t.Fatalf("SourceType = %q, want author", rec.SourceType)
	}
	if rec.SourceKey != "OL5345273A" {
		t.Fatalf("SourceKey = %q, want OL5345273A", rec.SourceKey)
	}
}

func TestReadSnapshotParsesMultipleLinesWithoutPanic(t *testing.T) {
	input := strings.Join([]string{
		`{"key":"/works/OL1W","title":"Work 1"}`,
		`{"key":"/works/OL2W","title":"Work 2"}`,
		"/type/author\t/authors/OL9A\t1\t2026-08-31T00:00:00Z\t{\"key\":\"/authors/OL9A\",\"name\":\"Name 9\",\"type\":{\"key\":\"/type/author\"}}",
	}, "\n") + "\n"

	records, errs := ReadSnapshot(strings.NewReader(input))
	count := 0
	for range records {
		count++
	}
	if err := <-errs; err != nil {
		t.Fatalf("ReadSnapshot() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("parsed record count = %d, want 3", count)
	}
}
