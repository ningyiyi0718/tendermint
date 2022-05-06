package main_test

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	metricsgen "github.com/tendermint/tendermint/scripts/metricsgen"
)

const testDataDir = "./testdata"

func TestSimpleTemplate(t *testing.T) {
	m := metricsgen.ParsedMetricField{
		TypeName:    "Histogram",
		FieldName:   "MyMetric",
		MetricName:  "request_count",
		Description: "how many requests were made since the start of the process",
		Labels:      "first, second, third",
	}
	td := metricsgen.TemplateData{
		Package:       "mypack",
		ParsedMetrics: []metricsgen.ParsedMetricField{m},
	}
	b := bytes.NewBuffer([]byte{})
	err := metricsgen.GenerateMetricsFile(b, td)
	if err != nil {
		t.Fatalf("unable to parse template %v", err)
	}
}

func TestFromData(t *testing.T) {
	infos, err := ioutil.ReadDir(testDataDir)
	if err != nil {
		t.Fatalf("unable to open file %v", err)
	}
	for _, dir := range infos {
		t.Run(dir.Name(), func(t *testing.T) {
			if !dir.IsDir() {
				t.Fatalf("expected file %s to be directory", dir.Name())
			}
			dirName := path.Join(testDataDir, dir.Name())
			pt, err := metricsgen.ParseMetricsDir(dirName, "Metrics")
			if err != nil {
				t.Fatalf("unable to parse from dir %q: %v", dir, err)
			}
			outFile := path.Join(dirName, "out.go")
			if err != nil {
				t.Fatalf("unable to open file %s: %v", outFile, err)
			}
			of, err := os.Create(outFile)
			if err != nil {
				t.Fatalf("unable to open file %s: %v", outFile, err)
			}
			defer os.Remove(outFile)
			if err := metricsgen.GenerateMetricsFile(of, pt); err != nil {
				t.Fatalf("unable to generate metrics file %s: %v", outFile, err)
			}
			if _, err := parser.ParseFile(token.NewFileSet(), outFile, nil, parser.AllErrors); err != nil {
				t.Fatalf("unable to parse generated file %s: %v", outFile, err)
			}
		})
	}
}

func TestParseMetricsStruct(t *testing.T) {
	const pkgName = "mypkg"
	metricsTests := []struct {
		name          string
		shouldError   bool
		metricsStruct string
		expected      metricsgen.TemplateData
	}{
		{
			name: "basic",
			metricsStruct: `type Metrics struct {
				myGauge metrics.Gauge
			}`,
			expected: metricsgen.TemplateData{
				Package: pkgName,
				ParsedMetrics: []metricsgen.ParsedMetricField{
					{
						TypeName:   "Gauge",
						FieldName:  "myGauge",
						MetricName: "my_gauge",
					},
				},
			},
		},
		{
			name: "histogram",
			metricsStruct: "type Metrics struct {\n" +
				"myHistogram metrics.Histogram `metricsgen_bucketsType:\"exp\" metricsgen_bucketSizes:\"1, 100, .8\"`\n" +
				"}",
			expected: metricsgen.TemplateData{
				Package: pkgName,
				ParsedMetrics: []metricsgen.ParsedMetricField{
					{
						TypeName:   "Histogram",
						FieldName:  "myHistogram",
						MetricName: "my_histogram",

						HistogramOptions: metricsgen.HistogramOpts{
							BucketType:  "stdprometheus.ExponentialBuckets",
							BucketSizes: "1, 100, .8",
						},
					},
				},
			},
		},
		{
			name: "labeled name",
			metricsStruct: "type Metrics struct {\n" +
				"myCounter metrics.Counter `metricsgen_name:\"new_name\"`\n" +
				"}",
			expected: metricsgen.TemplateData{
				Package: pkgName,
				ParsedMetrics: []metricsgen.ParsedMetricField{
					{
						TypeName:   "Counter",
						FieldName:  "myCounter",
						MetricName: "new_name",
					},
				},
			},
		},
		{
			name: "metric labels",
			metricsStruct: "type Metrics struct {\n" +
				"myCounter metrics.Counter `metricsgen_labels:\"label1, label2\"`\n" +
				"}",
			expected: metricsgen.TemplateData{
				Package: pkgName,
				ParsedMetrics: []metricsgen.ParsedMetricField{
					{
						TypeName:   "Counter",
						FieldName:  "myCounter",
						MetricName: "my_counter",
						Labels:     "label1, label2",
					},
				},
			},
		},
		{
			name: "ignore non-metric field",
			metricsStruct: `type Metrics struct {
				myCounter metrics.Counter
				nonMetric string
				}`,
			expected: metricsgen.TemplateData{
				Package: pkgName,
				ParsedMetrics: []metricsgen.ParsedMetricField{
					{
						TypeName:   "Counter",
						FieldName:  "myCounter",
						MetricName: "my_counter",
					},
				},
			},
		},
	}
	for _, testCase := range metricsTests {
		t.Run(testCase.name, func(t *testing.T) {
			dir, err := os.MkdirTemp(os.TempDir(), "metricsdir")
			if err != nil {
				t.Fatalf("unable to create directory: %v", err)
			}
			defer os.Remove(dir)
			f, err := os.Create(filepath.Join(dir, "metrics.go"))
			if err != nil {
				t.Fatalf("unable to open file: %v", err)
			}
			pkgLine := fmt.Sprintf("package %s\n", pkgName)
			importClause := `
			import(
				"github.com/go-kit/kit/metrics"
			)
			`

			f.Write([]byte(pkgLine))
			f.Write([]byte(importClause))
			f.Write([]byte(testCase.metricsStruct))

			td, err := metricsgen.ParseMetricsDir(dir, "Metrics")
			if testCase.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, testCase.expected, td)
			}
		})
	}
}
