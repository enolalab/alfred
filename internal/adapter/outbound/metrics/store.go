package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu       sync.RWMutex
	counters map[string]int64
	timings  map[string]timingAggregate
}

type timingAggregate struct {
	Count       int64   `json:"count"`
	TotalMs     float64 `json:"total_ms"`
	AverageMs   float64 `json:"average_ms"`
	LastValueMs float64 `json:"last_value_ms"`
}

func NewStore() *Store {
	return &Store{
		counters: make(map[string]int64),
		timings:  make(map[string]timingAggregate),
	}
}

func (s *Store) Inc(name string) {
	s.Add(name, 1)
}

func (s *Store) Add(name string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] += delta
}

func (s *Store) Set(name string, value int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters[name] = value
}

func (s *Store) ObserveDuration(name string, d time.Duration) {
	ms := float64(d) / float64(time.Millisecond)

	s.mu.Lock()
	defer s.mu.Unlock()

	agg := s.timings[name]
	agg.Count++
	agg.TotalMs += ms
	agg.LastValueMs = ms
	agg.AverageMs = agg.TotalMs / float64(agg.Count)
	s.timings[name] = agg
}

func (s *Store) Snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counters := make(map[string]int64, len(s.counters))
	for k, v := range s.counters {
		counters[k] = v
	}
	timings := make(map[string]timingAggregate, len(s.timings))
	for k, v := range s.timings {
		timings[k] = v
	}

	return map[string]any{
		"counters": counters,
		"timings":  timings,
	}
}

func (s *Store) PrometheusText() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var b strings.Builder

	counterNames := make([]string, 0, len(s.counters))
	for name := range s.counters {
		counterNames = append(counterNames, name)
	}
	sort.Strings(counterNames)

	for _, name := range counterNames {
		metricName := normalizePrometheusMetricName(name)
		metricType := "counter"
		if !strings.HasSuffix(name, "_total") {
			metricType = "gauge"
		}
		fmt.Fprintf(&b, "# TYPE %s %s\n", metricName, metricType)
		fmt.Fprintf(&b, "%s %d\n", metricName, s.counters[name])
	}

	timingNames := make([]string, 0, len(s.timings))
	for name := range s.timings {
		timingNames = append(timingNames, name)
	}
	sort.Strings(timingNames)

	for _, name := range timingNames {
		metricBase := normalizePrometheusMetricName(name)
		agg := s.timings[name]

		fmt.Fprintf(&b, "# TYPE %s_count counter\n", metricBase)
		fmt.Fprintf(&b, "%s_count %d\n", metricBase, agg.Count)
		fmt.Fprintf(&b, "# TYPE %s_total_ms gauge\n", metricBase)
		fmt.Fprintf(&b, "%s_total_ms %g\n", metricBase, agg.TotalMs)
		fmt.Fprintf(&b, "# TYPE %s_average_ms gauge\n", metricBase)
		fmt.Fprintf(&b, "%s_average_ms %g\n", metricBase, agg.AverageMs)
		fmt.Fprintf(&b, "# TYPE %s_last_value_ms gauge\n", metricBase)
		fmt.Fprintf(&b, "%s_last_value_ms %g\n", metricBase, agg.LastValueMs)
	}

	return b.String()
}

func normalizePrometheusMetricName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "alfred_metric"
	}

	var b strings.Builder
	b.Grow(len(name) + len("alfred_"))
	b.WriteString("alfred_")

	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	return b.String()
}
