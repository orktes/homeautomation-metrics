package types

import "time"

// Metric contains information for a single metric update
type Metric struct {
	Name   string
	Time   time.Time
	Fields map[string]interface{}
	Tags   map[string]string
}

// Clone returns a clone of the metric
func (m Metric) Clone() Metric {
	fields := map[string]interface{}{}
	for key, val := range m.Fields {
		fields[key] = val
	}

	tags := map[string]string{}
	for key, val := range m.Tags {
		tags[key] = val
	}

	m.Fields = fields
	m.Tags = tags

	return m
}
