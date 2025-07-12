package scanner

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"textfile-exporter/internal/collector"
	"textfile-exporter/internal/parser"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func isOlderThanTwoHours(t time.Time) bool {
	return time.Now().Sub(t) > 2*time.Hour
}

// Start runs the main loop of the scanner.
func Start(promPath string, oldFilesAge time.Duration, oldFilesExternalCmd string, scanInterval time.Duration, coll *collector.TimeAwareCollector) {
	for { // for ever
		fileinfo, err := os.Stat(promPath)
		if err != nil {
			log.Printf("Error stating path %s: %v\n", promPath, err)
			continue
		}
		var debugging bool

		// We have a simple runtime-switchable debug option.
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
			entries, err := os.ReadDir(promPath)
			if err != nil {
				log.Printf("Error reading directory %s: %v\n", promPath, err)
				continue
			}
			for _, entry := range entries {
				fi, err := entry.Info()
				if err != nil {
					continue
				}
				if fi.IsDir() {
					continue
				}
				name := fi.Name()
				if fi.Mode().IsRegular() && strings.HasSuffix(name, ".prom") {
					files = append(files, promPath+"/"+fi.Name())
				}
			}
		} else {
			files = append(files, promPath)
		}
		n := len(files)
		log.Printf("Found %d files\n", n)

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

			if time.Now().After(fileinfo.ModTime().Add(oldFilesAge)) {
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
						fmt.Println("output:\n<<<\n" + string(cmdOut) + ">>>")
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
