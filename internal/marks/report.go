package marks

import (
	"fmt"
	"sort"
	"strings"
)

// Human renders a Report for a terminal.
func (r Report) Human() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Length: %d chars\n", r.Length)
	fmt.Fprintf(&b, "Suspicious: %d\n", r.SuspiciousTotal)
	if len(r.Hits) > 0 {
		b.WriteString("Hits:\n")
		for _, h := range r.Hits {
			fmt.Fprintf(&b, "  [%s/%s] %s x%d @ %v\n",
				h.Kind, h.Confidence, h.Label, h.Count,
				h.SampleOffsets[:min(len(h.SampleOffsets), 5)])
		}
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&b, "Note: %s\n", n)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Human renders Stats as a one-line summary, or a detailed breakdown.
func (s Stats) Human(detailed bool) string {
	head := fmt.Sprintf("removed=%d replaced=%d len %d->%d",
		s.RemovedCount, s.ReplacedCount, s.InputLength, s.OutputLength)
	if s.VisibleDropped > 0 || s.VisibleTransliterated > 0 || s.VisibleSpacesFolded > 0 {
		head += fmt.Sprintf(" visible: dropped=%d transliterated=%d spaces=%d",
			s.VisibleDropped, s.VisibleTransliterated, s.VisibleSpacesFolded)
	}
	if s.NFKCChanged {
		head += " (NFKC applied)"
	}
	if !detailed {
		return head
	}
	var b strings.Builder
	b.WriteString(head)
	writeCounts(&b, "removed", s.Removed)
	writeCounts(&b, "replaced", s.Replaced)
	return b.String()
}

func writeCounts(b *strings.Builder, heading string, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	fmt.Fprintf(b, "\n  %s:", heading)
	for _, k := range keys {
		fmt.Fprintf(b, "\n    %s x%d", k, counts[k])
	}
}
