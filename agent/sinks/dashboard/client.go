package dashboard

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/NorthlandPowerEurope/dashy/agent/sinks/value"
)

type Client struct {
	address string
	token   string
	timeout time.Duration
	mapping map[string]string
}

func NewClient(params map[string]string, mapping map[string]string) (*Client, error) {
	address := params["address"]
	if address == "" {
		return nil, errors.New("address is required")
	}
	token := params["token"]
	if token == "" {
		return nil, errors.New("token is required")
	}
	timeout := 10 * time.Second
	if t, ok := params["timeout"]; ok {
		s, err := strconv.Atoi(t)
		if err != nil {
			return nil, errors.New("invalid timeout value")
		}
		timeout = time.Duration(s) * time.Second
	}
	return &Client{
		address: address,
		token:   token,
		timeout: timeout,
		mapping: mapping,
	}, nil
}

func (c *Client) Send(values value.Container) error {

	b, err := json.Marshal(values.Map(c.mapping))
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.address, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: c.timeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return errors.New("invalid token (403)")
	}
	if resp.StatusCode == 400 {
		return errors.New("invalid request (400)")
	}
	if resp.StatusCode != 200 {
		return errors.New("unexpected error: " + resp.Status)
	}
	return nil
}
