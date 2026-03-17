// internal/ingester/rss_test.go
package ingester_test

import (
	"os"
	"testing"

	"github.com/youmnarabie/poo/internal/ingester"
)

func TestParseRSS_MurderShack(t *testing.T) {
	data, _ := os.ReadFile("../../testdata/feed_murder_shack.xml")
	feed, err := ingester.ParseRSS(data)
	if err != nil {
		t.Fatalf("ParseRSS: %v", err)
	}
	if feed.Title != "Test Fiction Podcast" {
		t.Errorf("got title %q", feed.Title)
	}
	if len(feed.Episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(feed.Episodes))
	}
	if feed.Episodes[0].AudioURL == "" {
		t.Error("missing audio URL")
	}
}

func TestParseRSS_SkipsMissingEnclosure(t *testing.T) {
	xml := `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title>
		<item><guid>g1</guid><title>No audio</title></item>
	</channel></rss>`
	feed, err := ingester.ParseRSS([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Episodes) != 0 {
		t.Errorf("expected 0 episodes, got %d", len(feed.Episodes))
	}
}
