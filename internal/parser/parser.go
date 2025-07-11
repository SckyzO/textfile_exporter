package parser

import (
	"os"
	"path/filepath"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// ParseMF parses the file at the passed path and returns a map of metric families.
func ParseMF(path string) (map[string]*dto.MetricFamily, error) {
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
	defer reader.Close()

	// We parse the content to return the metrics family result.
	var parser expfmt.TextParser
	mf, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return nil, err
	}
	return mf, nil
}
