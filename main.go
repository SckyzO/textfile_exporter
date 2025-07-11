package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

var (
	// Build-time variables
	version   string
	revision  string
	branch    string
	buildUser string
	buildDate string
	goVersion string

	webListenAddress = kingpin.Flag(
		"web.listen-address",
		"Address on which to expose metrics and web interface.",
	).Default(":9014").String()
	webConfigFile = kingpin.Flag(
		"web.config.file",
		"[NOT IMPLEMENTED] Path to configuration file that can enable TLS or authentication.",
	).String()
	promPath = kingpin.Flag(
		"textfile.directory",
		"Path for prom file or dir of *.prom files.",
	).Default(".").String()
	scanInterval = kingpin.Flag(
		"scan-interval",
		"The interval at which to scan the directory for .prom files.",
	).Default("30s").Duration()
	memoryMaxAge = kingpin.Flag(
		"memory-max-age",
		"Max age of in-memory metrics.",
	).Default("25h").Duration()
	oldFilesAge = kingpin.Flag(
		"files-min-age",
		"Min age of files to be considered old.",
	).Default("6h").Duration()
	oldFilesExternalCmd = kingpin.Flag(
		"old-files-external-command",
		"External command to execute on old files. The filename is passed as the last argument.",
	).Default("ls -l").String()
)

func fatal(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

// This is going to parse the file at the passed path.
func parseMF(path string) (map[string]*dto.MetricFamily, error) {

	// Standard (overkill?) path sanification.
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		path = filepath.Clean(string(os.PathSeparator) + path)
		path, _ = filepath.Rel(string(os.PathSeparator), path)
	}
	path = filepath.Clean(path)

	// We open the path.
	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// We parse the content to return the metrics family result.
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}
	return mf, nil
}

func isOlderThanTwoHours(t time.Time) bool {
	return time.Now().Sub(t) > 2*time.Hour
}

// Main function here.
func main() {
	kingpin.Version(fmt.Sprintf(
		"textfile-exporter, version %s (branch: %s, revision: %s)\n  build user: %s\n  build date: %s\n  go version: %s\n  platform: %s\n",
		version, branch, revision, buildUser, buildDate, goVersion, runtime.GOOS+"/"+runtime.GOARCH,
	))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *webConfigFile != "" {
		log.Println("Warning: --web.config.file is not implemented and will be ignored.")
	}

	// Create our collector.
	collector := newTimeAwareCollector(*memoryMaxAge)

	// Start a background job to constantly watch for files and parse them.
	go func() {
		log.Printf("Textfile Exporter started\n")
		for { // for ever
			file_path := *promPath
			fileinfo, err := os.Stat(file_path)
			if err != nil {
				log.Printf("Error stating path %s: %v\n", file_path, err)
				continue
			}
			var debugging bool

			// We have a simple runtime-switchable debug option.
			// If this file exists and is not older then two hours, debug
			// output is enabled.
			if fs, err := os.Stat(file_path + "/debug_tfe"); err == nil {
				if !isOlderThanTwoHours(fs.ModTime()) {
					debugging = true
				} else {
					debugging = false
				}
			} else {
				debugging = false
			}
			if debugging {
				log.Printf("*** DEBUG MODE ENABLED ***\n")
			}

			// Let's collect a list of files to process.
			var files []string
			// For a dir, we process the contained files named "*.prom".
			if fileinfo.IsDir() {
				entries, err := os.ReadDir(file_path)
				if err != nil {
					log.Printf("Error reading directory %s: %v\n", file_path, err)
					continue
				}
				for _, entry := range entries {
					fi, err := entry.Info()
					if err != nil {
						continue
					}
					if fi.IsDir() {
						continue // We do not do recursion.
					}
					name := fi.Name()
					if fi.Mode().IsRegular() && strings.HasSuffix(name, ".prom") {
						files = append(files, file_path+"/"+fi.Name())
					}
				}
			} else {
				files = append(files, file_path)
			}
			n := len(files)
			log.Printf("Found %d files\n", n)

			// We create a new map for the metrics we are about to parse.
			newMetrics := make(map[string]storedMetric)

			// Parse the files.
			for i, f := range files {
				printIt := debugging || i < 5 || i >= n-5
				if printIt {
					log.Printf("%d/%d Processing file %s\n", i+1, n, f)
				}
				// check age
				fileinfo, err := os.Stat(f)
				if err != nil {
					log.Printf("%d/%d Error stat()ing file %s\n", i+1, n, f)
					continue
				}
				// Old files are ignored and a specified external script may be run.
				if time.Now().After(fileinfo.ModTime().Add(*oldFilesAge)) {
					log.Printf("%d/%d Old file %s\n", i+1, n, f)
					parts := strings.Fields(*oldFilesExternalCmd)
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
                        fmt.Println("output:\n<<<\n" + string(cmdOut) + ">>>")
                    }
                    continue
				}
				// Actual parsing.
				mfs, err := parseMF(f)
				if err != nil {
					log.Printf("%d/%d Error parsing file %s\n", i+1, n, f)
					continue
				}

				// Handle parsing results.
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

						// Handle the timestamp.
						timestamp := m.GetTimestampMs()
						if debugging {
							log.Println("  Metric Value: ", metric_value)
							log.Println("  Timestamp: ", timestamp)
						}
						if timestamp <= 0 { // We generate a timestamp if it is missing.
							timestamp = time.Now().UTC().UnixNano() / 1000000
							if debugging {
								log.Println("  Timestamp: ", timestamp, " (now)")
							}
						}

						// Handle the labels.
						for _, label := range m.GetLabel() {
							if debugging {
								log.Println("  Label_Name:  ", label.GetName())
								log.Println("  Label_Value: ", label.GetValue())
							}
							labels[label.GetName()] = label.GetValue()
						}

						// Add the metric into the temporary map.
						fullname, metric := collector.Add(name, labels, metric_type, metric_value, time.Unix(0, timestamp*int64(time.Millisecond)), 0, mf.GetHelp())
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
			// We replace the old metrics map with the new one.
			collector.ReplaceMetrics(newMetrics)
			time.Sleep(*scanInterval)

		}

	}()

	// Register ourselves.
	r := prometheus.NewRegistry()
	r.MustRegister(collector)
	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)
	http.HandleFunc("/alive", aliveAnswer)

	// Configure the http server and start it.
	s := &http.Server{
		Addr:           *webListenAddress,
		Handler:        nil,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}

// This can be called by liveness probes, a lot better than
// invoking the /metrics endpoint and generate output that
// will be ignored.
func aliveAnswer(w http.ResponseWriter, req *http.Request) {
	log.Println("confirming i'm alive")
	fmt.Fprintf(w, "i'm alive\n")
}
