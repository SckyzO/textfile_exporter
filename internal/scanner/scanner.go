package scanner

import (
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"textfile_exporter/internal/collector"
	"textfile_exporter/internal/parser"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func isOlderThanTwoHours(t time.Time) bool {
	return time.Now().Sub(t) > 2*time.Hour
}

// Start begins the scanning loop that periodically reads .prom files from a
// directory, parses the metrics, and updates the collector. This function is
// intended to be run as a goroutine.
//
// - promPath: The directory or file to scan for metrics.
// - recursive: Whether to scan the directory recursively.
// - enableFilesMinAge: Flag to enable checking for old files.
// - filesMinAgeDuration: Duration to consider a file old.
// - oldFilesExternalCmd: Command to run on old files.
// - scanInterval: How often to scan the directory.
// - coll: The TimeAwareCollector to which metrics will be added.
// - scannedFilesCount: A gauge to update with the number of files found.
// - lastScanTimestamp: A gauge to update with the timestamp of the last scan.
func Start(promPath string, recursive bool, enableFilesMinAge bool, filesMinAgeDuration time.Duration, oldFilesExternalCmd string, scanInterval time.Duration, coll *collector.TimeAwareCollector, scannedFilesCount prometheus.Gauge, lastScanTimestamp prometheus.Gauge) {
	for {
		lastScanTimestamp.SetToCurrentTime()
		fileinfo, err := os.Stat(promPath)
		if err != nil {
			log.Printf("Error stating path %s: %v\n", promPath, err)
			continue
		}
		var debugging bool

		// Enable debug logging if a 'debug_tfe' file exists and is recent.
		if fs, err := os.Stat(promPath + "/debug_tfe"); err == nil {
			if !isOlderThanTwoHours(fs.ModTime()) {
				debugging = true
			}
		}
		if debugging {
			log.Printf("*** DEBUG MODE ENABLED ***\n")
		}

		var files []string
		if fileinfo.IsDir() {
			if recursive {
				err := filepath.WalkDir(promPath, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() && strings.HasSuffix(d.Name(), ".prom") {
						files = append(files, path)
					}
					return nil
				})
				if err != nil {
					log.Printf("Error walking directory %s: %v\n", promPath, err)
					continue
				}
			} else {
				entries, err := os.ReadDir(promPath)
				if err != nil {
					log.Printf("Error reading directory %s: %v\n", promPath, err)
					continue
				}
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".prom") {
						files = append(files, filepath.Join(promPath, entry.Name()))
					}
				}
			}
		} else {
			files = append(files, promPath)
		}
		n := len(files)
		log.Printf("Found %d files\n", n)
		scannedFilesCount.Set(float64(n))

		newMetrics := make(map[string]collector.StoredMetric)

		for i, f := range files {
			printIt := debugging || i < 5 || i >= n-5
			if printIt {
				log.Printf("%d/%d Processing file %s\n", i+1, n, f)
			}
			fileinfo, err := os.Stat(f)
			if err != nil {
				log.Printf("%d/%d Error stat()ing file %s\n", i+1, n, f)
				continue
			}
			mfs, err := parser.ParseMF(f)
			if err != nil {
				log.Printf("%d/%d Error parsing file %s\n", i+1, n, f)
				continue
			}

			// If enabled, execute an external command on files older than the specified duration.
			if enableFilesMinAge && time.Now().After(fileinfo.ModTime().Add(filesMinAgeDuration)) {
				log.Printf("%d/%d Old file %s\n", i+1, n, f)
				parts := strings.Fields(oldFilesExternalCmd)
				if len(parts) > 0 {
					cmd_to_run := parts[0]
					cmd_args := parts[1:]
					cmd_args = append(cmd_args, f)
					cmd := exec.Command(cmd_to_run, cmd_args...)
					log.Printf("%d/%d Running command %s\n", i+1, n, cmd.String())
					cmdOut, err := cmd.Output()
					if err != nil {
						log.Printf("%d/%d Error running command %s\n", i+1, n, cmd.String())
					}
					if debugging {
						log.Printf("output:\n<<<\n%s\n>>>\n", string(cmdOut))
					}
				}
			}

			cnt := 0
			for name, mf := range mfs {
				labels := make(map[string]string)
				if debugging {
					log.Println("Metric Name: ", name)
					log.Println("Metric Type: ", mf.GetType())
					log.Println("Metric Help: ", mf.GetHelp())
				}

				var metric_value float64
				var metric_type prometheus.ValueType
			out:
				for _, m := range mf.GetMetric() {
					switch mf.GetType() {
					case dto.MetricType_GAUGE:
						metric_type = prometheus.GaugeValue
						metric_value = m.GetGauge().GetValue()
					case dto.MetricType_COUNTER:
						metric_type = prometheus.CounterValue
						metric_value = m.GetCounter().GetValue()
					case dto.MetricType_SUMMARY:
						break out
					case dto.MetricType_UNTYPED:
						metric_type = prometheus.UntypedValue
						metric_value = m.GetUntyped().GetValue()
					case dto.MetricType_HISTOGRAM:
						break out
					default:
						break out
					}

					timestamp := m.GetTimestampMs()
					if debugging {
						log.Println("  Metric Value: ", metric_value)
						log.Println("  Timestamp: ", timestamp)
					}
					// If the metric has no timestamp, assign the current time.
					if timestamp <= 0 {
						timestamp = time.Now().UTC().UnixNano() / 1000000
						if debugging {
							log.Println("  Timestamp: ", timestamp, " (now)")
						}
					}

					for _, label := range m.GetLabel() {
						if debugging {
							log.Println("  Label_Name:  ", label.GetName())
							log.Println("  Label_Value: ", label.GetValue())
						}
						labels[label.GetName()] = label.GetValue()
					}

					fullname, metric := coll.CreateMetric(name, labels, metric_type, metric_value, time.Unix(0, timestamp*int64(time.Millisecond)), 0, mf.GetHelp())
					newMetrics[fullname] = metric
					cnt++

					if debugging {
						log.Println("-----------")
					}
				}
			}
			if printIt {
				log.Printf("%d/%d    found %d data points\n", i+1, n, cnt)
			}
		}
		coll.ReplaceMetrics(newMetrics)
		time.Sleep(scanInterval)
	}
}

