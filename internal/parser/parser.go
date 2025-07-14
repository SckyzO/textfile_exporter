package parser

import (
	"os"
	"path/filepath"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// ParseMF reads a file in the Prometheus text exposition format and parses it
// into a map of MetricFamily protocol buffer items.
func ParseMF(path string) (map[string]*dto.MetricFamily, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		path = filepath.Clean(string(os.PathSeparator) + path)
		path, _ = filepath.Rel(string(os.PathSeparator), path)
	}
	path = filepath.Clean(path)

	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}
	return mf, nil
}
