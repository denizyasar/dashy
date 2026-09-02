package sources

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"4d63.com/tz"
)

const downloadPHP = "/download.php?file="

type RADACAdapter struct {
	params   map[string]string
	paths    []string
	factors  []float64
	timezone *time.Location
}

func NewRADACAdapter(params map[string]string) (*RADACAdapter, error) {
	err := checkParams(params, "address", "paths", "factors", "timezone")
	if err != nil {
		return nil, err
	}
	timezone, err := tz.LoadLocation(params["timezone"])
	if err != nil {
		return nil, err
	}

	paths := strings.Split(params["paths"], ";")
	factorStrings := strings.Split(params["factors"], ";")
	if len(paths) != len(factorStrings) {
		return nil, errors.New("the number of paths needs to be the same as the number of factors")
	}

	factors := []float64{}
	for _, factorString := range factorStrings {
		factor, err := strconv.ParseFloat(factorString, 64)
		if err != nil {
			return nil, errors.New("each factor needs to be a floating point number")
		}
		factors = append(factors, factor)
	}

	return &RADACAdapter{
		params:   params,
		paths:    paths,
		factors:  factors,
		timezone: timezone,
	}, nil
}

func (a *RADACAdapter) Values() (map[string]Value, error) {
	values := make(map[string]Value)
	for i := range a.paths {
		b, err := a.retrieve(a.paths[i])
		if err != nil {
			return nil, err
		}
		ts, valStr, err := a.ExtractLastEntry(bytes.NewBuffer(b))
		if err != nil {
			return nil, err
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err == nil {
			valStr = fmt.Sprintf("%.2f", val*a.factors[i])
		}
		values[a.paths[i]] = Value{
			Timestamp: ts,
			Value:     valStr,
		}
	}
	return values, nil
}

func (a *RADACAdapter) Close() error {
	return nil
}

func (a *RADACAdapter) createLink(path string) string {
	return a.params["address"] + downloadPHP + path + time.Now().In(a.timezone).Format("20060102") + ".txt"
}

func (a *RADACAdapter) retrieve(path string) ([]byte, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("GET", a.createLink(path), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, errors.New("body not present")
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func (a *RADACAdapter) ExtractLastEntry(r io.Reader) (time.Time, string, error) {
	lastLine := ""
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return time.Time{}, "", err
	}

	lastLineSplit := strings.Split(lastLine, ",")
	if len(lastLineSplit) != 2 {
		return time.Time{}, "", errors.New("malformed last line entry")
	}
	dayOffset, err := strconv.Atoi(lastLineSplit[0])
	if err != nil {
		return time.Time{}, "", errors.New("malformed last line entry")
	}

	return time.Now().Truncate(24 * time.Hour).Add(time.Duration(dayOffset) * time.Millisecond), lastLineSplit[1], nil
}
