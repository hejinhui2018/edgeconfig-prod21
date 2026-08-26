package persistence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"edgeconfig/domain"
)

func TestFileStoreRecoversValidRecordsAndIgnoresDamagedTail(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	event, err := domain.NewEnvelope(1, "event-one", domain.EventSiteCreated, "site-a", domain.Site{ID: "site-a", Name: "A", CreatedAt: now}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Append(context.Background(), 0, []domain.Envelope{event}); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString(`{"schema_version":1,"sequence":2`); err != nil {
		t.Fatal(err)
	}
	file.Close()
	reopened, err := OpenFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	events, report, err := reopened.ReadAfter(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || report.LastSequence != 1 || report.IgnoredTailBytes == 0 {
		t.Fatalf("unexpected recovery: events=%d report=%+v", len(events), report)
	}
}

func TestFileStoreRejectsSequenceGap(t *testing.T) {
	dir := t.TempDir()
	line := `{"schema_version":1,"sequence":2,"id":"event-two","type":"site.created","aggregate_id":"site-a","occurred_at":"2026-01-01T00:00:00Z","data":{"id":"site-a","name":"A","created_at":"2026-01-01T00:00:00Z"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(line), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFileStore(dir); err == nil {
		t.Fatal("expected sequence validation failure")
	}
}
