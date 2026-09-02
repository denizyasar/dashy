package sources

import (
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"
	"time"

	"4d63.com/tz"
)

var (
	recentTimestampFormat = "2006-01-02T15:04:05.000"
)

type recentValues struct {
	Tags struct {
		Tag []struct {
			ID        string `xml:"id,attr"`
			Code      string `xml:"code,attr"`
			Name      string `xml:"name,attr"`
			Unit      string `xml:"unit,attr"`
			Timestamp string `xml:"timestamp,attr"`
			Timealarm string `xml:"timealarm,attr"`
			Modflags  string `xml:"modflags,attr"`
			Target    string `xml:"target,attr"`
			Value     string `xml:"_value,attr"`
			Min       string `xml:"min,attr"`
			Lolo      string `xml:"lolo,attr"`
			Lo        string `xml:"lo,attr"`
			Hi        string `xml:"hi,attr"`
			Hihi      string `xml:"hihi,attr"`
			Max       string `xml:"max,attr"`
		} `xml:"tag"`
	} `xml:"tags"`
}

type OmcAdapter struct {
	params   map[string]string
	timezone *time.Location
}

func (a *OmcAdapter) Close() error {
	return nil
}

func NewOmcAdapter(params map[string]string) (*OmcAdapter, error) {
	err := checkParams(params, "address", "username", "password", "timezone")
	if err != nil {
		return nil, err
	}
	timezone, err := tz.LoadLocation(params["timezone"])
	if err != nil {
		return nil, err
	}
	return &OmcAdapter{
		params:   params,
		timezone: timezone,
	}, nil
}

func (a *OmcAdapter) httpGET() (io.ReadCloser, error) {
	client := &http.Client{
		CheckRedirect: a.redirectPolicyFunc,
		Timeout:       10 * time.Second,
	}

	req, err := http.NewRequest("GET", a.params["address"], nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+a.basicAuth())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (a *OmcAdapter) redirectPolicyFunc(req *http.Request, via []*http.Request) error {
	req.Header.Add("Authorization", "Basic "+a.basicAuth())
	return nil
}

func (a *OmcAdapter) basicAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(a.params["username"] + ":" + a.params["password"]))
}

func (a *OmcAdapter) Values() (map[string]Value, error) {

	body, err := a.httpGET()
	if err != nil {
		return nil, err
	}
	defer body.Close()

	values, err := parseRecentValues(body, a.timezone)
	if err != nil {
		return nil, err
	}

	return values, nil
}

func parseRecentValues(r io.ReadCloser, timezone *time.Location) (map[string]Value, error) {
	var recentValues recentValues

	err := xml.NewDecoder(r).Decode(&recentValues)
	if err != nil {
		return nil, err
	}

	values := make(map[string]Value)

	for _, tag := range recentValues.Tags.Tag {
		if tag.Timestamp == "" {
			continue
		}

		timestamp, err := time.ParseInLocation(recentTimestampFormat, tag.Timestamp, timezone)
		if err != nil {
			return nil, err
		}
		timestamp = timestamp.Truncate(time.Second)

		value := tag.Value
		if tag.Unit != "" {
			value += " " + tag.Unit
		}
		values[tag.Name] = Value{
			Timestamp: timestamp,
			Value:     value,
		}
	}

	return values, nil
}
