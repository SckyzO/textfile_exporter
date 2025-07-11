package collector

import (
	"log"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// A prometheus metric plus timestamps for insertion and expiration.
type StoredMetric struct {
	InsertionTime  time.Time
	PromMetric     *prometheus.Metric
	ExpirationTime time.Time
}

// A container of metrics (with the important Collect function that outputs them).
type TimeAwareCollector struct {
	metrics               map[string]StoredMetric
	metricsMutex          sync.Mutex // This is used to serialize write access to the metrics map.
	defaultExpireDuration time.Duration
}

// Constructor
func NewTimeAwareCollector(expire time.Duration) *TimeAwareCollector {
	return &TimeAwareCollector{
		metrics:               make(map[string]StoredMetric),
		defaultExpireDuration: expire,
	}
}

// Mandatory function, it must emit description of metrics.
func (c *TimeAwareCollector) Describe(ch chan<- *prometheus.Desc) {
	// We use the lock to avoid any change while enumeration is in progress.
	c.metricsMutex.Lock() // take lock
	for _, metric := range c.metrics {
		ch <- (*metric.PromMetric).Desc()
	}
	c.metricsMutex.Unlock() //release lock
}

// Mandatory function, it must emit metrics.
func (c *TimeAwareCollector) Collect(ch chan<- prometheus.Metric) {
	// We are being asked to emit our metrics content.
	// Some metrics may have been now expired and should not be emitted,
	// they should also be discarded from our memory,
	// so we first do a cleanup and then emit all the surviving ones.

	begin := time.Now()
	log.Printf("\n")
	var expiredKeys []string
	var localMap = make(map[string]StoredMetric)

	// We use the lock to avoid any change while the expiration analysis is in progress.
	c.metricsMutex.Lock() // take lock

	// Searching for expired metrics.
	// Instead of directly deleting them, we create a list and then
	// delete the ones in the list; we avoid changing the map
	// during the analysis (even if deletion inside a range loop
	// is considered safe).
	for k, metric := range c.metrics {
		if time.Now().After(metric.ExpirationTime) {
			//log.Printf("deleted met ts=%v\n", metric.insertionTime)
			expiredKeys = append(expiredKeys, k)
		} else {
			//log.Printf("        met ts=%v\n", metric.insertionTime)
			localMap[k] = metric
		}
	}
	// This is the real deletion, still happening with lock taken.
	for _, k := range expiredKeys {
		delete(c.metrics, k)
	}

	// We now release the lock since from this point we operate read-only.
	c.metricsMutex.Unlock() //release lock

	// Finally emit the metrics.
	// This may take some time, we are implicitly writing into a TCP socket
	// which could block.
	// This may run concurrently with itself and with map changing operations.
	n := 0
	for _, metric := range localMap {
		//log.Printf("                                 emitted met ts=%v\n", metric.insertionTime)
		n++
		ch <- *metric.PromMetric
	}
	log.Printf("emitted %d metrics in %f seconds\n", n, time.Now().Sub(begin).Seconds())
}

// Unusual chars are not desired.
var specialCharsRegex = regexp.MustCompile("[^a-zA-Z0-9]+")

// This is used to add a new metric to the collector.
func (c *TimeAwareCollector) CreateMetric(name string, labels map[string]string, promtype prometheus.ValueType, value float64, timestamp time.Time, expireDuration time.Duration, description string) (string, StoredMetric) {

	// Each metric has a map of labels (key,value).
	// We do a copy of what we have been passed, just to sanitize the names of the label.
	labelMap := make(map[string]string)
	for k, v := range labels {
		labelMap[specialCharsRegex.ReplaceAllString(k, "_")] = v
	}

	// Create a sorted list of the label names.
	var keys []string
	for k := range labelMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create parallel arrays of label names and values, as needed
	// by "prometheus." functions.
	// At the same time create a metric name string with all labels keys and
	// values appended. It is going to be used as the key for the main metrics map;
	// this means that new values will overwrite old ones only if the entire
	// name+labels string is the same, e.g.
	//   cpu[host="abc"] 0.15 1690000044
	// will overwite
	//   cpu[host="abc"] 0.18 1690000013
	// but not ovewrite
	//   cpu[host="xyz"] 0.03 1690000022
	// which is in a different logical metrics sequence.
	labelNames := make([]string, 0)
	labelValues := make([]string, 0)
	fullname := name
	for _, k := range keys {
		labelNames = append(labelNames, k)
		labelValues = append(labelValues, labelMap[k])
		fullname = fullname + "|" + k + "|" + labelMap[k]
	}

	// Finally create the metric object as by "prometheus." code.
	desc := prometheus.NewDesc(name, description, labelNames, nil)
	promMetric := prometheus.MustNewConstMetric(desc, promtype, value, labelValues...)
	promMetric = prometheus.NewMetricWithTimestamp(timestamp, promMetric)

	// And wrap it in our metrics structure.
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

// This is used to clear the collector (discard all metrics).
func (c *TimeAwareCollector) ReplaceMetrics(newMetrics map[string]StoredMetric) {
	c.metricsMutex.Lock() // take lock
	c.metrics = newMetrics
	c.metricsMutex.Unlock() //release lock
}
