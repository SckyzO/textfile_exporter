package collector

import (
	"log"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// StoredMetric is a wrapper around a Prometheus metric that includes timestamps
// for its insertion and expiration.
type StoredMetric struct {
	InsertionTime  time.Time
	PromMetric     *prometheus.Metric
	ExpirationTime time.Time
}

// TimeAwareCollector is a custom Prometheus collector that stores metrics in memory
// and manages their lifecycle, including garbage collection of expired metrics.
type TimeAwareCollector struct {
	metrics               map[string]StoredMetric
	metricsMutex          sync.Mutex
	defaultExpireDuration time.Duration
}

// NewTimeAwareCollector creates and returns a new TimeAwareCollector.
// It requires a default expiration duration for the metrics it will store.
func NewTimeAwareCollector(expire time.Duration) *TimeAwareCollector {
	return &TimeAwareCollector{
		metrics:               make(map[string]StoredMetric),
		defaultExpireDuration: expire,
	}
}

// Describe implements the prometheus.Collector interface. It sends the descriptions
// of all stored metrics to the provided channel.
func (c *TimeAwareCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metricsMutex.Lock()
	defer c.metricsMutex.Unlock()
	for _, metric := range c.metrics {
		ch <- (*metric.PromMetric).Desc()
	}
}

// Collect implements the prometheus.Collector interface. It is called by the
// Prometheus registry to gather metrics. It first removes expired metrics and
// then sends the remaining metrics to the provided channel.
func (c *TimeAwareCollector) Collect(ch chan<- prometheus.Metric) {
	begin := time.Now()
	var expiredKeys []string
	var localMap = make(map[string]StoredMetric)

	c.metricsMutex.Lock()

	// First, identify expired metrics without modifying the map while iterating.
	for k, metric := range c.metrics {
		if time.Now().After(metric.ExpirationTime) {
			expiredKeys = append(expiredKeys, k)
		} else {
			localMap[k] = metric
		}
	}

	// Now, remove the expired metrics from the main map.
	for _, k := range expiredKeys {
		delete(c.metrics, k)
	}

	c.metricsMutex.Unlock()

	// Finally, emit the surviving metrics. This is done outside the lock to
	// avoid blocking other operations while writing to the channel.
	for _, metric := range localMap {
		ch <- *metric.PromMetric
	}
	log.Printf("emitted %d metrics in %f seconds\n", len(localMap), time.Now().Sub(begin).Seconds())
}

// specialCharsRegex is used to sanitize label keys.
var specialCharsRegex = regexp.MustCompile("[^a-zA-Z0-9]+")

// CreateMetric builds a new Prometheus metric from the provided data and wraps
// it in a StoredMetric. It returns a unique key for the metric and the metric itself.
// The key is a combination of the metric name and its sorted labels, ensuring that
// each time series is unique.
func (c *TimeAwareCollector) CreateMetric(name string, labels map[string]string, promtype prometheus.ValueType, value float64, timestamp time.Time, expireDuration time.Duration, description string) (string, StoredMetric) {
	// Sanitize label keys to conform to Prometheus standards.
	labelMap := make(map[string]string)
	for k, v := range labels {
		labelMap[specialCharsRegex.ReplaceAllString(k, "_")] = v
	}

	// Create a sorted list of label names to ensure consistent key generation.
	var keys []string
	for k := range labelMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Generate the unique metric key (fullname) by concatenating the name and sorted labels.
	// This ensures that `cpu{host="b"}` and `cpu{host="a"}` are treated as distinct series.
	labelNames := make([]string, 0)
	labelValues := make([]string, 0)
	fullname := name
	for _, k := range keys {
		labelNames = append(labelNames, k)
		labelValues = append(labelValues, labelMap[k])
		fullname = fullname + "|" + k + "|" + labelMap[k]
	}

	// Create the Prometheus metric.
	desc := prometheus.NewDesc(name, description, labelNames, nil)
	promMetric := prometheus.MustNewConstMetric(desc, promtype, value, labelValues...)
	promMetric = prometheus.NewMetricWithTimestamp(timestamp, promMetric)

	// Wrap it in our StoredMetric structure with expiration info.
	var metric StoredMetric
	metric.InsertionTime = time.Now().UTC()
	metric.PromMetric = &promMetric
	if expireDuration > 0 {
		metric.ExpirationTime = time.Now().UTC().Add(expireDuration)
	} else {
		metric.ExpirationTime = time.Now().UTC().Add(c.defaultExpireDuration)
	}

	return fullname, metric
}

// ReplaceMetrics atomically replaces the entire set of stored metrics with a new map.
func (c *TimeAwareCollector) ReplaceMetrics(newMetrics map[string]StoredMetric) {
	c.metricsMutex.Lock()
	c.metrics = newMetrics
	c.metricsMutex.Unlock()
}
