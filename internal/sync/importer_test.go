package sync_test

import (
	"context"
	"testing"

	"github.com/nanotube/nanotube-web/internal/storage"
	"github.com/nanotube/nanotube-web/internal/sync"
)

func TestImportSubscriptions_Formats(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenMemory(ctx)
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	if err := storage.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	defer db.Close()
	repo := storage.NewRepository(db)

	// 1. OPML XML
	opmlData := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="1.0">
  <body>
    <outline title="Canal Go" text="Canal Go" xmlUrl="https://www.youtube.com/feeds/videos.xml?channel_id=UC_golang123" />
    <outline title="Tech News" text="Tech News" xmlUrl="https://www.youtube.com/feeds/videos.xml?channel_id=UC_technews456" />
  </body>
</opml>`
	res, err := sync.ImportSubscriptions(ctx, repo, []byte(opmlData))
	if err != nil {
		t.Fatalf("import OPML: %v", err)
	}
	if res.Imported != 2 {
		t.Errorf("expected 2 imported, got %d", res.Imported)
	}

	// 2. CSV Google Takeout
	csvData := `Channel Id,Channel Url,Channel Title
UC_takeout789,http://www.youtube.com/channel/UC_takeout789,Takeout Channel
UC_another999,http://www.youtube.com/channel/UC_another999,Another Channel`
	res, err = sync.ImportSubscriptions(ctx, repo, []byte(csvData))
	if err != nil {
		t.Fatalf("import CSV: %v", err)
	}
	if res.Imported != 2 {
		t.Errorf("expected 2 imported from CSV, got %d", res.Imported)
	}

	// 3. NewPipe JSON
	jsonData := `{
  "subscriptions": [
    {
      "url": "https://www.youtube.com/channel/UC_newpipe111",
      "name": "NewPipe Channel"
    }
  ]
}`
	res, err = sync.ImportSubscriptions(ctx, repo, []byte(jsonData))
	if err != nil {
		t.Fatalf("import JSON: %v", err)
	}
	if res.Imported != 1 {
		t.Errorf("expected 1 imported from JSON, got %d", res.Imported)
	}

	// Verify in repository
	channels, err := repo.SubscribedChannels(ctx)
	if err != nil {
		t.Fatalf("SubscribedChannels: %v", err)
	}
	if len(channels) < 5 {
		t.Errorf("expected at least 5 channels in repo, got %d", len(channels))
	}
}
