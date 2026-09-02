package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Dashboard struct {
	mutex sync.Mutex

	// Ident of this dashboard used to create the url. (e.g. something.de/dashboard)
	Ident string `json:"ident"`

	// Title of the page (e.g. DBU Wind Data)
	Title string `json:"title"`

	// Interval defines how often the dashboard gets refreshed.
	Refresh int `json:"refresh"`

	// NoData defines after how many seconds the dashboard shows a no data warning.
	NoData int `json:"nodata"`

	// Variables hold all the variables.
	Variables [][]*Variable `json:"variables"`

	// Authentication maps origin addresses to access tokens and is used to authenticate each POST request containing
	// new data.
	Authentication map[string]string `json:"authentication"`

	// ---

	// Timestamp holds a date string of the last update.
	Timestamp time.Time `json:"timestamp,omitempty"`
}

func (d *Dashboard) Status() Status {
	if d.Timestamp.IsZero() || d.NoData <= 0 {
		return StatusNeutral
	}
	if (int(time.Now().Sub(d.Timestamp).Seconds()) / d.NoData) > 1 {
		return StatusDanger
	}
	return StatusSuccess
}

func (d *Dashboard) FormattedTimestamp() string {
	if d.Timestamp.IsZero() {
		return "never"
	}
	return d.Timestamp.UTC().Format(TIMESTAMP_FORMAT)
}

func (d *Dashboard) Update(w http.ResponseWriter, r *http.Request) {

	// authenticate request with origin and token
	requestToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	remoteAddr := strings.Split(r.RemoteAddr, ":")
	if requestToken == "" || len(remoteAddr) != 2 {
		http.Error(w, "invalid request", 400)
		return
	}
	isAuthenticated := false
	for addr, token := range d.Authentication {
		if remoteAddr[0] == addr && token == requestToken {
			isAuthenticated = true
		}
	}
	if !isAuthenticated {
		http.Error(w, "unauthorized", 401)
		return
	}

	// its fine to lock later, tokens are read only
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// read values
	if r.Body == nil {
		http.Error(w, "call requires request body", 400)
		return
	}
	defer r.Body.Close()
	values := make(map[string]Value)
	err := json.NewDecoder(r.Body).Decode(&values)
	if err != nil {
		http.Error(w, "request body must be of type json", 400)
		return
	}

	// update values, trim history if required
	for x := range d.Variables {
		for y := range d.Variables[x] {
			if newValue, exist := values[d.Variables[x][y].Ident]; exist {
				d.Variables[x][y].Value = newValue
				d.Variables[x][y].History = append([]Value{newValue}, d.Variables[x][y].History...)
				if len(d.Variables[x][y].History) > d.Variables[x][y].Capacity {
					d.Variables[x][y].History = d.Variables[x][y].History[:d.Variables[x][y].Capacity]
				}
			}
		}
	}

	// OK
	d.Timestamp = time.Now()
	w.WriteHeader(200)
}

func (d *Dashboard) Render(w http.ResponseWriter) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return DASHBOARD_TEMPLATE.Execute(w, d)
}
