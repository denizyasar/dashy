package server

import (
	"net/http"
	"time"
)

type Variable struct {
	Dashboard *Dashboard

	// Ident of this measurement used to create urls. (e.g. something.de/dashboard/windspeed)
	Ident string `json:"ident"`

	// Title holds the measurement title.
	Title string `json:"title"`

	// Subtitle holds the measurement subtitle.
	Subtitle string `json:"subtitle"`

	// Unit appended to the value
	Unit string `json:"unit,omitempty"`

	NoData int `json:"nodata"`

	Capacity int `json:"capacity"`

	// ---

	Value Value `json:"value,omitempty"`

	// ---

	History []Value `json:"history,omitempty"`
}

func (v *Variable) Status() Status {
	if v.Value.Timestamp.IsZero() || v.NoData <= 0 {
		return StatusNeutral
	}
	if (int(time.Now().Sub(v.Value.Timestamp).Seconds()) / v.NoData) > 1 {
		return StatusDanger
	}
	return StatusSuccess
}

func (v *Variable) RenderHistory(w http.ResponseWriter) error {
	v.Dashboard.mutex.Lock()
	defer v.Dashboard.mutex.Unlock()

	return HISTORY_TEMPLATE.Execute(w, v)
}
