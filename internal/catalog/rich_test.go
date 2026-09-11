package catalog

import (
	"context"
	"testing"
)

func TestGetRichWork_ReturnsSeriesAudioAndPublicAssets(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	rich, err := repo.GetRichWork(context.Background(), WorkIDDune)
	if err != nil {
		t.Fatalf("GetRichWork: %v", err)
	}
	if rich.Work.WorkID != WorkIDDune {
		t.Fatalf("expected work %s, got %s", WorkIDDune, rich.Work.WorkID)
	}
	if len(rich.SeriesMembership) == 0 {
		t.Fatal("expected at least one series membership")
	}
	if len(rich.Audio) < 2 {
		t.Fatalf("expected at least two audio performance rows, got %d", len(rich.Audio))
	}
	for _, asset := range rich.Assets {
		if asset.Eligibility != "public" {
			t.Fatalf("expected only public assets, got eligibility=%q", asset.Eligibility)
		}
	}
}

func TestCompareEditions_DetectsDifferences(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	cmp, err := repo.CompareEditions(context.Background(), EdIDDunePrint, EdIDDuneEbook)
	if err != nil {
		t.Fatalf("CompareEditions: %v", err)
	}
	if !cmp.SameWork {
		t.Fatal("expected compared editions to belong to the same work")
	}
	if len(cmp.Differences) == 0 {
		t.Fatal("expected at least one difference")
	}
}

func TestQualityReport_HasFormatAndSourceCoverage(t *testing.T) {
	db := openTestDB(t)
	repo := NewSQLRepository(db)

	report, err := repo.QualityReport(context.Background())
	if err != nil {
		t.Fatalf("QualityReport: %v", err)
	}
	if len(report.ByFormatLanguage) == 0 {
		t.Fatal("expected quality rows by format/language")
	}
	if len(report.BySource) == 0 {
		t.Fatal("expected quality rows by source")
	}
}
