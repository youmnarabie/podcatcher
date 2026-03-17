// internal/ingester/detector.go
package ingester

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Rule struct {
	Pattern  string
	Priority int
}

type DetectResult struct {
	SeriesName    string
	EpisodeNumber *int
}

func DetectSeries(title string, rules []Rule) *DetectResult {
	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })

	for _, rule := range sorted {
		re, err := regexp.Compile("(?i)" + rule.Pattern)
		if err != nil {
			continue
		}
		match := re.FindStringSubmatch(title)
		if match == nil {
			continue
		}
		result := &DetectResult{}
		for i, name := range re.SubexpNames() {
			if i >= len(match) {
				break
			}
			switch name {
			case "series":
				result.SeriesName = strings.TrimSpace(match[i])
			case "number":
				trimmed := strings.TrimLeft(match[i], "0")
				if trimmed == "" {
					trimmed = "0"
				}
				if n, err := strconv.Atoi(trimmed); err == nil {
					result.EpisodeNumber = &n
				}
			}
		}
		if result.SeriesName != "" {
			return result
		}
	}
	return nil
}

func SameSeriesName(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
