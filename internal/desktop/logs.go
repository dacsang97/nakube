package desktop

import (
	"hash/fnv"
	"regexp"

	"github.com/egoist/mygo/ui"
)

// LogLine is one owned log line tagged with its pod.
type LogLine struct {
	Pod  string
	Text string
}

var podPalette = []ui.Color{
	ui.Hex("#2563eb"),
	ui.Hex("#059669"),
	ui.Hex("#d97706"),
	ui.Hex("#dc2626"),
	ui.Hex("#7c3aed"),
	ui.Hex("#0891b2"),
	ui.Hex("#be185d"),
	ui.Hex("#4d7c0f"),
}

// PodColor is stable for a pod name.
func PodColor(name string) ui.Color {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return podPalette[h.Sum32()%uint32(len(podPalette))]
}

// FilterLines hides lines that do not match pattern. An empty pattern keeps every line.
func FilterLines(lines []LogLine, pattern string) ([]LogLine, error) {
	if pattern == "" {
		out := make([]LogLine, len(lines))
		copy(out, lines)
		return out, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	out := make([]LogLine, 0, len(lines))
	for _, line := range lines {
		if re.MatchString(line.Text) || re.MatchString(line.Pod) {
			out = append(out, line)
		}
	}
	return out, nil
}
