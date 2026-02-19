package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Stats tracks persistent usage data across sessions.
type Stats struct {
	TunnelsBuilt int `json:"tunnels_built"`
}

// Milestone messages keyed by tunnel count thresholds.
var milestones = map[int]string{
	100:   "100 tunnels. You might have a problem.",
	500:   "500 tunnels. At this point you ARE the network.",
	1000:  "1000 tunnels. Legend.",
	10000: "10000 tunnels. They should name a protocol after you.",
}

// milestoneThresholds in ascending order for crossing detection.
var milestoneThresholds = []int{100, 500, 1000, 10000}

func statsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tunneler", "stats.json")
}

// Load reads the stats file. Returns zero stats if the file doesn't exist.
func Load() Stats {
	data, err := os.ReadFile(statsPath())
	if err != nil {
		return Stats{}
	}
	var s Stats
	if err := json.Unmarshal(data, &s); err != nil {
		return Stats{}
	}
	return s
}

// save writes stats to disk, creating the directory if needed.
func save(s Stats) error {
	p := statsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// AddTunnels increments the tunnel counter and saves. Returns a milestone
// message if a threshold was just crossed, or empty string otherwise.
func AddTunnels(count int) string {
	s := Load()
	prev := s.TunnelsBuilt
	s.TunnelsBuilt += count
	_ = save(s) // best-effort, don't break the app if this fails

	// Check if we crossed a milestone.
	for _, threshold := range milestoneThresholds {
		if prev < threshold && s.TunnelsBuilt >= threshold {
			return milestones[threshold]
		}
	}
	return ""
}
