package usageevent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	sqliterepo "github.com/seakee/cpa-manager-plus/apps/manager-server/internal/repository/sqlite"
	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/usage"
)

func TestEventsPageClassifiesRecoveredInternalRetriesAcrossPages(t *testing.T) {
	db, err := sqliterepo.Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo := New(db)
	base := time.Date(2026, time.July, 31, 10, 0, 0, 0, time.UTC)
	events := []usage.Event{
		retryEvent("failed-attempt-1", "request-recovered", "key-a", base, true),
		retryEvent("failed-attempt-2", "request-recovered", "key-a", base.Add(time.Second), true),
		retryEvent("recovered-success", "request-recovered", "key-a", base.Add(2*time.Second), false),
		retryEvent("later-session-success", "request-recovered", "key-a", base.Add(30*time.Second), false),
		retryEvent("terminal-failure", "request-terminal", "key-a", base.Add(3*time.Second), true),
		retryEvent("normal-success", "request-normal", "key-a", base.Add(4*time.Second), false),
		retryEvent("different-key-success", "request-terminal", "key-b", base.Add(5*time.Second), false),
		retryEvent("late-success", "request-terminal", "key-a", base.Add(3*time.Minute), false),
	}
	if _, err := repo.InsertBatch(context.Background(), events); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	filter := AnalyticsFilter{
		FromMS:        base.Add(-time.Second).UnixMilli(),
		ToMS:          base.Add(10 * time.Minute).UnixMilli(),
		IncludeFailed: true,
	}
	firstPage, err := repo.EventsPageWithFilter(context.Background(), filter, 0, 0, 2)
	if err != nil {
		t.Fatalf("first events page: %v", err)
	}
	if !firstPage.HasMore || len(firstPage.Items) != 2 {
		t.Fatalf("first page = %#v", firstPage)
	}
	allItems := append([]EventPageItem(nil), firstPage.Items...)

	beforeMS := firstPage.NextBeforeMS
	beforeID := firstPage.NextBeforeID
	for {
		page, err := repo.EventsPageWithFilter(context.Background(), filter, beforeMS, beforeID, 2)
		if err != nil {
			t.Fatalf("next events page: %v", err)
		}
		allItems = append(allItems, page.Items...)
		if !page.HasMore {
			break
		}
		beforeMS = page.NextBeforeMS
		beforeID = page.NextBeforeID
	}

	byHash := make(map[string]EventPageItem, len(allItems))
	for _, item := range allItems {
		byHash[item.EventHash] = item
	}
	for _, eventHash := range []string{"failed-attempt-1", "failed-attempt-2"} {
		item, exists := byHash[eventHash]
		if !exists {
			t.Fatalf("%s missing from page results: %#v", eventHash, allItems)
		}
		if !item.InternalRetryRecovered || item.RecoveredAfterRetry {
			t.Fatalf("%s retry flags = recovered:%v success:%v", eventHash, item.InternalRetryRecovered, item.RecoveredAfterRetry)
		}
	}
	if item := byHash["recovered-success"]; item.InternalRetryRecovered || !item.RecoveredAfterRetry {
		t.Fatalf("recovered success retry flags = recovered:%v success:%v", item.InternalRetryRecovered, item.RecoveredAfterRetry)
	}
	for _, eventHash := range []string{"later-session-success", "terminal-failure", "normal-success", "different-key-success", "late-success"} {
		item := byHash[eventHash]
		if item.InternalRetryRecovered || item.RecoveredAfterRetry {
			t.Fatalf("%s unexpectedly classified as retry recovery: %#v", eventHash, item)
		}
	}
}

func retryEvent(eventHash, requestID, apiKeyHash string, timestamp time.Time, failed bool) usage.Event {
	return usage.Event{
		RequestID:    requestID,
		EventHash:    eventHash,
		TimestampMS:  timestamp.UnixMilli(),
		Timestamp:    timestamp.Format(time.RFC3339Nano),
		Provider:     "openai",
		ExecutorType: "CodexExecutor",
		Model:        "gpt-test",
		Endpoint:     "POST /v1/responses",
		Method:       "POST",
		Path:         "/v1/responses",
		APIKeyHash:   apiKeyHash,
		Failed:       failed,
		CreatedAtMS:  timestamp.UnixMilli(),
	}
}
