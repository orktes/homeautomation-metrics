package collector

import (
	"github.com/armon/go-metrics"
	"github.com/orktes/homeautomation-metrics/internal/types"
)

// Collector collects prometheus metrics from a given chan types.Metric
type Collector struct {
	metricChan <-chan types.Metric
	closeChan  chan struct{}
	metricSink metrics.MetricSink
}

// New returns a new collector
func New(metricChan <-chan types.Metric, metricSink metrics.MetricSink) *Collector {
	c := &Collector{metricChan: metricChan, closeChan: make(chan struct{}), metricSink: metricSink}
	go c.read()
	return c
}

func (c *Collector) read() {
	for {
		select {
		case m := <-c.metricChan:
			c.updateMetrics(m)
		case <-c.closeChan:
			return
		}
	}
}

func (c *Collector) updateMetrics(m types.Metric) {
	labels := convertTagsToLabels(m.Tags)

	for field, value := range m.Fields {
		if strVal, ok := value.(string); ok {
			nameLabels := append([]metrics.Label{
				metrics.Label{
					Name:  "value",
					Value: strVal,
				},
			}, labels...)
			c.metricSink.SetGaugeWithLabels([]string{m.Name, field}, 1, nameLabels)
			continue
		}
		promVal := convertToPromValue(value)
		c.metricSink.SetGaugeWithLabels([]string{m.Name, field}, promVal, labels)
	}
}

// Close closes metrics collector
func (c *Collector) Close() error {
	close(c.closeChan)
	return nil
}

func convertTagsToLabels(tags map[string]string) (labels []metrics.Label) {
	for key, val := range tags {
		labels = append(labels, metrics.Label{
			Name:  key,
			Value: val,
		})
	}

	return
}

func convertToPromValue(val interface{}) float32 {
	switch v := val.(type) {
	case bool:
		if v {
			return 1
		}
		return 0
	case float64:
		return float32(v)
	}

	return 0
}
