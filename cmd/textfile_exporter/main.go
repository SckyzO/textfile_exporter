package main

import (
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"textfile_exporter/internal/collector"
	"textfile_exporter/internal/scanner"
	"textfile_exporter/internal/webconfig"
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
		"Path to configuration file that can enable TLS or authentication.",
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

// Internal metrics exposed by the exporter itself.
var (
	scannedFilesCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "textfile_exporter_scanned_files_count",
		Help: "Number of .prom files found in the last scan.",
	})
	lastScanTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "textfile_exporter_last_scan_timestamp",
		Help: "Unix timestamp of the last scan.",
	})
)

// indexHTML is the HTML content for the root page.
const indexHTML = `<html>
<head><title>Textfile Exporter</title></head>
<body>
<h1>Textfile Exporter</h1>
<p>Click <a href='/metrics'>here</a> to see the metrics.</p>
</body>
</html>`

// basicAuthMiddleware wraps a handler to enforce basic authentication.
func basicAuthMiddleware(handler http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized.", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// main is the entrypoint of the application.
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

	var webConfig *webconfig.WebConfig
	if *webConfigFile != "" {
		var err error
		webConfig, err = webconfig.LoadConfig(*webConfigFile)
		if err != nil {
			log.Fatalf("Failed to load web config: %v", err)
		}
	}

	coll := collector.NewTimeAwareCollector(*memoryMaxAge)

	go scanner.Start(*promPath, *enableFilesMinAge, *filesMinAgeDuration, *oldFilesExternalCmd, *scanInterval, coll, scannedFilesCount, lastScanTimestamp)

	r := prometheus.NewRegistry()
	r.MustRegister(coll)
	r.MustRegister(scannedFilesCount)
	r.MustRegister(lastScanTimestamp)
	r.MustRegister(prometheus.NewGoCollector())
	r.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	metricsHandler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	indexHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(indexHTML))
	})

	if webConfig != nil && webConfig.BasicAuth != nil && webConfig.BasicAuth.Username != "" && webConfig.BasicAuth.PasswordFile != "" {
		password, err := ioutil.ReadFile(webConfig.BasicAuth.PasswordFile)
		if err != nil {
			log.Fatalf("Failed to read password file: %v", err)
		}
		passwordStr := strings.TrimSpace(string(password))
		metricsHandler = basicAuthMiddleware(metricsHandler, webConfig.BasicAuth.Username, passwordStr)
		http.Handle("/metrics", metricsHandler)
		http.Handle("/", basicAuthMiddleware(indexHandler, webConfig.BasicAuth.Username, passwordStr))
		log.Println("Basic authentication is enabled.")
	} else {
		http.Handle("/metrics", metricsHandler)
		http.Handle("/", indexHandler)
	}

	s := &http.Server{
		Addr:           *webListenAddress,
		Handler:        nil,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if webConfig != nil && webConfig.TLS != nil && webConfig.TLS.CertFile != "" && webConfig.TLS.KeyFile != "" {
		tlsConfig := &tls.Config{}

		if webConfig.TLS.ClientCAFile != "" {
			caCert, err := ioutil.ReadFile(webConfig.TLS.ClientCAFile)
			if err != nil {
				log.Fatalf("Failed to read client CA file: %v", err)
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			tlsConfig.ClientCAs = caCertPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			log.Println("Client certificate authentication is enabled.")
		}

		s.TLSConfig = tlsConfig
		log.Printf("Listening with TLS...")
		log.Fatal(s.ListenAndServeTLS(webConfig.TLS.CertFile, webConfig.TLS.KeyFile))
	} else {
		log.Printf("Listening without TLS...")
		log.Fatal(s.ListenAndServe())
	}
}