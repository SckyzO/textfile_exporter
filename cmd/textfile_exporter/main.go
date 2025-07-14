package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"textfile_exporter/internal/collector"
	"textfile_exporter/internal/scanner"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Build-time variables
	version   string
	revision  string
	branch    string
	buildUser string
	buildDate string
	goVersion string
	projectURL string

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
	enableFilesMinAge = kingpin.Flag(
		"files-min-age",
		"Enable or disable the minimum age check for files. If enabled, files older than 'files-min-age-duration' will be considered old.",
	).Default("true").Bool()
	filesMinAgeDuration = kingpin.Flag(
		"files-min-age-duration",
		"Minimum age of files to be considered old, if 'files-min-age' is enabled.",
	).Default("6h").Duration()
	oldFilesExternalCmd = kingpin.Flag(
		"old-files-external-command",
		"External command to execute on old files. The filename is passed as the last argument.",
	).Default("ls -l").String()
	logLevel = kingpin.Flag(
		"log.level",
		"Only log messages with the given severity or above. One of: [debug, info, warn, error]",
	).Default("info").String()
)

const indexHTML = `<html>
<head><title>Textfile Exporter</title></head>
<body>
<h1>Textfile Exporter</h1>
<p>Click <a href='/metrics'>here</a> to see the metrics.</p>
</body>
</html>`

func main() {
	kingpin.Version(fmt.Sprintf(
		"textfile_exporter, version %s (branch: %s, revision: %s)\n  build user: %s\n  build date: %s\n  go version: %s\n  platform: %s\n  project url: %s\n",
		version, branch, revision, buildUser, buildDate, goVersion, runtime.GOOS+"/"+runtime.GOARCH, projectURL,
	))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Printf("Starting textfile_exporter version %s", version)
	log.Printf("Listen address: %s", *webListenAddress)
	log.Printf("Metrics path: %s", *promPath)
	log.Printf("Scan interval: %s", (*scanInterval).String())
	log.Printf("Max metric age: %s", (*memoryMaxAge).String())
	log.Printf("Enable file min age check: %t", *enableFilesMinAge)
	log.Printf("Min file age duration: %s", (*filesMinAgeDuration).String())
	log.Printf("Cleanup command: %s", *oldFilesExternalCmd)

	if *webConfigFile != "" {
		log.Println("Warning: --web.config.file is not implemented and will be ignored.")
	}

	coll := collector.NewTimeAwareCollector(*memoryMaxAge)

	go scanner.Start(*promPath, *enableFilesMinAge, *filesMinAgeDuration, *oldFilesExternalCmd, *scanInterval, coll)

	r := prometheus.NewRegistry()
	r.MustRegister(coll)
	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(indexHTML))
	})

	s := &http.Server{
		Addr:           *webListenAddress,
		Handler:        nil,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}