package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"textfile-exporter/internal/collector"
	"textfile-exporter/internal/scanner"
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

	coll := collector.NewTimeAwareCollector(*memoryMaxAge)

	go scanner.Start(*promPath, *oldFilesAge, *oldFilesExternalCmd, *scanInterval, coll)

	r := prometheus.NewRegistry()
	r.MustRegister(coll)
	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)
	http.HandleFunc("/alive", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "i'm alive\n")
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