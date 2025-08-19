package metrics

import (
	"context"
	"sync"
	"time"
)

// Metric represents a single metric measurement
type Metric struct {
	Name      string                 `json:"name"`
	Value     interface{}            `json:"value"`
	Timestamp time.Time              `json:"timestamp"`
	Tags      map[string]string      `json:"tags,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// SystemMetrics contains aggregated system performance metrics
type SystemMetrics struct {
	AvgResponseTime int64     `json:"avg_response_time"`
	P95ResponseTime int64     `json:"p95_response_time"`
	P99ResponseTime int64     `json:"p99_response_time"`
	ResponseTimes   []int64   `json:"response_times"`
	MessageFlow     MessageFlow `json:"message_flow"`
	Timestamp       time.Time `json:"timestamp"`
}

// MessageFlow tracks message statistics
type MessageFlow struct {
	Sent       int64 `json:"sent"`
	Received   int64 `json:"received"`
	Processing int64 `json:"processing"`
}

// Collector aggregates and provides access to system metrics
type Collector struct {
	mu           sync.RWMutex
	metrics      []Metric
	maxMetrics   int
	systemStats  *SystemMetrics
	lastUpdate   time.Time
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		metrics:    make([]Metric, 0, 1000),
		maxMetrics: 1000, // Keep last 1000 metrics
		systemStats: &SystemMetrics{
			ResponseTimes: make([]int64, 0, 20),
			MessageFlow: MessageFlow{},
			Timestamp:   time.Now(),
		},
		lastUpdate: time.Now(),
	}
}

// RecordMetric adds a new metric measurement
func (c *Collector) RecordMetric(metric Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	metric.Timestamp = time.Now()
	c.metrics = append(c.metrics, metric)

	// Keep only the most recent metrics
	if len(c.metrics) > c.maxMetrics {
		c.metrics = c.metrics[len(c.metrics)-c.maxMetrics:]
	}

	// Update system stats based on metric type
	c.updateSystemStats(metric)
}

// RecordResponseTime records an API response time
func (c *Collector) RecordResponseTime(duration time.Duration) {
	ms := duration.Milliseconds()
	c.RecordMetric(Metric{
		Name:  "response_time",
		Value: ms,
		Tags:  map[string]string{"type": "api"},
	})
}

// RecordMessageSent increments sent message counter
func (c *Collector) RecordMessageSent() {
	c.RecordMetric(Metric{
		Name:  "message_sent",
		Value: 1,
		Tags:  map[string]string{"type": "count"},
	})
}

// RecordMessageReceived increments received message counter
func (c *Collector) RecordMessageReceived() {
	c.RecordMetric(Metric{
		Name:  "message_received",
		Value: 1,
		Tags:  map[string]string{"type": "count"},
	})
}

// RecordMessageProcessing increments processing message counter
func (c *Collector) RecordMessageProcessing() {
	c.RecordMetric(Metric{
		Name:  "message_processing",
		Value: 1,
		Tags:  map[string]string{"type": "count"},
	})
}

// GetSystemMetrics returns current aggregated system metrics
func (c *Collector) GetSystemMetrics(ctx context.Context) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy of current stats
	return map[string]interface{}{
		"avg_response_time": c.systemStats.AvgResponseTime,
		"p95_response_time": c.systemStats.P95ResponseTime,
		"p99_response_time": c.systemStats.P99ResponseTime,
		"response_times":    append([]int64{}, c.systemStats.ResponseTimes...),
		"message_flow": map[string]interface{}{
			"sent":       c.systemStats.MessageFlow.Sent,
			"received":   c.systemStats.MessageFlow.Received,
			"processing": c.systemStats.MessageFlow.Processing,
		},
		"timestamp": c.systemStats.Timestamp,
	}
}

// GetMetrics returns recent metrics with optional filtering
func (c *Collector) GetMetrics(ctx context.Context, filter map[string]string, limit int) []Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var filtered []Metric
	count := 0

	// Start from the end (most recent)
	for i := len(c.metrics) - 1; i >= 0 && count < limit; i-- {
		metric := c.metrics[i]
		
		// Apply filters
		if c.matchesFilter(metric, filter) {
			filtered = append(filtered, metric)
			count++
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	return filtered
}

// updateSystemStats aggregates metrics into system statistics
func (c *Collector) updateSystemStats(metric Metric) {
	now := time.Now()
	
	switch metric.Name {
	case "response_time":
		if ms, ok := metric.Value.(int64); ok {
			// Add to response times (keep last 20)
			c.systemStats.ResponseTimes = append(c.systemStats.ResponseTimes, ms)
			if len(c.systemStats.ResponseTimes) > 20 {
				c.systemStats.ResponseTimes = c.systemStats.ResponseTimes[1:]
			}

			// Calculate percentiles
			c.calculateResponseTimeStats()
		}

	case "message_sent":
		c.systemStats.MessageFlow.Sent++

	case "message_received":
		c.systemStats.MessageFlow.Received++

	case "message_processing":
		c.systemStats.MessageFlow.Processing++
	}

	c.systemStats.Timestamp = now
	c.lastUpdate = now
}

// calculateResponseTimeStats computes avg, p95, p99 from recent response times
func (c *Collector) calculateResponseTimeStats() {
	times := c.systemStats.ResponseTimes
	if len(times) == 0 {
		return
	}

	// Make a copy and sort for percentile calculation
	sorted := make([]int64, len(times))
	copy(sorted, times)
	
	// Simple sort (bubble sort is fine for small arrays)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate average
	var sum int64
	for _, t := range times {
		sum += t
	}
	c.systemStats.AvgResponseTime = sum / int64(len(times))

	// Calculate percentiles
	n := len(sorted)
	c.systemStats.P95ResponseTime = sorted[int(float64(n)*0.95)]
	c.systemStats.P99ResponseTime = sorted[int(float64(n)*0.99)]
}

// matchesFilter checks if a metric matches the given filter criteria
func (c *Collector) matchesFilter(metric Metric, filter map[string]string) bool {
	if filter == nil || len(filter) == 0 {
		return true
	}

	// Check name filter
	if name, exists := filter["name"]; exists && metric.Name != name {
		return false
	}

	// Check tag filters
	for key, value := range filter {
		if key == "name" {
			continue // Already checked above
		}
		
		if tagValue, exists := metric.Tags[key]; !exists || tagValue != value {
			return false
		}
	}

	return true
}

// Reset clears all collected metrics
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics = make([]Metric, 0, c.maxMetrics)
	c.systemStats = &SystemMetrics{
		ResponseTimes: make([]int64, 0, 20),
		MessageFlow:   MessageFlow{},
		Timestamp:     time.Now(),
	}
	c.lastUpdate = time.Now()
}

// GetLastUpdateTime returns when metrics were last updated
func (c *Collector) GetLastUpdateTime() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUpdate
}