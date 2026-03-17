// internal/ingester/detector_test.go
package ingester_test

import (
	"testing"

	"github.com/youmnarabie/poo/internal/ingester"
)

var defaultRules = []ingester.Rule{
	{Pattern: `(?P<series>.+?)\s+(?P<number>\d+)\s+FINALE:`, Priority: 1},
	{Pattern: `(?P<series>.+?)\s+FINALE:`, Priority: 2},
	{Pattern: `(?P<series>.+?)\s+(?P<number>\d+):`, Priority: 3},
	{Pattern: `^(?P<number>\d+)\s+(?P<series>.+)$`, Priority: 4},
}

func TestDetect_SeriesWithNumber(t *testing.T) {
	r := ingester.DetectSeries("The Murder Shack 03: 'Benedict Cumberbatch'", defaultRules)
	if r == nil {
		t.Fatal("expected match")
	}
	if r.SeriesName != "The Murder Shack" {
		t.Errorf("got %q", r.SeriesName)
	}
	if r.EpisodeNumber == nil || *r.EpisodeNumber != 3 {
		t.Errorf("got number %v", r.EpisodeNumber)
	}
}

func TestDetect_FinaleWithNumber(t *testing.T) {
	r := ingester.DetectSeries("The Murder Shack 06 FINALE: 'Closure?'", defaultRules)
	if r == nil || r.SeriesName != "The Murder Shack" {
		t.Fatalf("got %+v", r)
	}
	if r.EpisodeNumber == nil || *r.EpisodeNumber != 6 {
		t.Errorf("got number %v", r.EpisodeNumber)
	}
}

func TestDetect_FinaleNoNumber(t *testing.T) {
	r := ingester.DetectSeries("Vengeance From Beyond FINALE: 'The Ties That Bind'", defaultRules)
	if r == nil || r.SeriesName != "Vengeance From Beyond" {
		t.Fatalf("got %+v", r)
	}
	if r.EpisodeNumber != nil {
		t.Errorf("expected no number, got %d", *r.EpisodeNumber)
	}
}

func TestDetect_PrefixNumber(t *testing.T) {
	r := ingester.DetectSeries("03 Working for the Washington Brothers", defaultRules)
	if r == nil || r.SeriesName != "Working for the Washington Brothers" {
		t.Fatalf("got %+v", r)
	}
	if r.EpisodeNumber == nil || *r.EpisodeNumber != 3 {
		t.Errorf("got number %v", r.EpisodeNumber)
	}
}

func TestDetect_SingleDigitPrefix(t *testing.T) {
	r := ingester.DetectSeries("3 Working for the Washington Brothers", defaultRules)
	if r == nil || r.EpisodeNumber == nil || *r.EpisodeNumber != 3 {
		t.Fatalf("got %+v", r)
	}
}

func TestDetect_CaseInsensitiveDedup(t *testing.T) {
	r1 := ingester.DetectSeries("Miami Mince—Yule Regret It 01: 'Eggnog'", defaultRules)
	r2 := ingester.DetectSeries("Miami Mince—Yule Regret it 02: 'Holly'", defaultRules)
	if r1 == nil || r2 == nil {
		t.Fatal("expected both to match")
	}
	if !ingester.SameSeriesName(r1.SeriesName, r2.SeriesName) {
		t.Errorf("%q vs %q should be same series", r1.SeriesName, r2.SeriesName)
	}
}

func TestDetect_NoMatch(t *testing.T) {
	if r := ingester.DetectSeries("Just a random episode title", defaultRules); r != nil {
		t.Errorf("expected no match, got %+v", r)
	}
}
