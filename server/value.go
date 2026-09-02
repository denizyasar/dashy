package server

import "time"

type Value struct {
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value"`
}

func (v *Value) FormattedTimestamp() string {
	if v.Timestamp.IsZero() {
		return "never"
	}
	return v.Timestamp.UTC().Format(TIMESTAMP_FORMAT)
}

func (v *Value) FormattedValue() string {
	if v.Value == "" {
		return "-"
	}
	return v.Value
}
