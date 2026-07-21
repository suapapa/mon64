package collector

import (
	"fmt"
	"io"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// parseMetrics reads Prometheus text exposition into a map keyed by metric name.
func parseMetrics(r io.Reader) (map[string]float64, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(r)
	if err != nil {
		return nil, fmt.Errorf("parse metrics: %w", err)
	}
	out := make(map[string]float64)
	for name, family := range families {
		switch family.GetType() {
		case dto.MetricType_GAUGE, dto.MetricType_COUNTER, dto.MetricType_UNTYPED:
		default:
			continue
		}
		for _, m := range family.GetMetric() {
			key := metricKey(name, m)
			val, ok := sampleValue(m)
			if !ok {
				continue
			}
			out[key] = val
		}
	}
	return out, nil
}

func metricKey(name string, m *dto.Metric) string {
	if len(m.GetLabel()) == 0 {
		return name
	}
	var key strings.Builder
	key.WriteString(name + "{")
	first := true
	for _, lp := range m.GetLabel() {
		if !first {
			key.WriteString(",")
		}
		first = false
		key.WriteString(lp.GetName() + "=\"" + lp.GetValue() + "\"")
	}
	key.WriteString("}")
	return key.String()
}

func sampleValue(m *dto.Metric) (float64, bool) {
	switch {
	case m.Gauge != nil:
		return m.Gauge.GetValue(), true
	case m.Counter != nil:
		return m.Counter.GetValue(), true
	case m.Untyped != nil:
		return m.Untyped.GetValue(), true
	default:
		return 0, false
	}
}

// gaugeByName returns the first gauge/counter value for an unlabeled metric name.
func gaugeByName(metrics map[string]float64, name string) (float64, bool) {
	v, ok := metrics[name]
	return v, ok
}

// labeledValues returns all values whose key starts with name+"{".
func labeledValues(metrics map[string]float64, name string) map[string]float64 {
	prefix := name + "{"
	out := make(map[string]float64)
	for k, v := range metrics {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out[k] = v
		}
	}
	return out
}

// labelValue extracts a label value from a metric key like name{cpu="0",mode="idle"}.
func labelValue(key, label string) (string, bool) {
	needle := label + "=\""
	idx := strings.IndexByte(key, '{')
	if idx < 0 {
		return "", false
	}
	rest := key[idx+1:]
	start := strings.Index(rest, needle)
	if start < 0 {
		return "", false
	}
	rest = rest[start+len(needle):]
	end := strings.Index(rest, "\"")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}
