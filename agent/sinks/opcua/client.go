package opcua

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/NorthlandPowerEurope/dashy/agent/sinks/value"
	"github.com/NorthlandPowerEurope/dashy/opcua/client"
	"github.com/NorthlandPowerEurope/dashy/opcua/ua"
	"github.com/rs/zerolog/log"
)

type DataType uint32

const (
	Float64 DataType = 11
	Uint16  DataType = 5
	String  DataType = 12
)

func (dt DataType) String() string {
	switch dt {
	case Float64:
		return "Float64"
	case Uint16:
		return "Uint16"
	case String:
		return "String"
	default:
		return "Unknown"
	}
}

func (dt DataType) Parse(value string) (interface{}, error) {
	if dt == String {
		return value, nil
	}
	matches := regexp.MustCompile(`(?m)\d+([\.|\,]\d+)?`).FindAllStringSubmatch(value, 1)
	if len(matches) != 1 {
		return nil, fmt.Errorf("invalid value format: %s", value)
	}
	s := strings.Replace(matches[0][0], ",", ".", 1)
	switch dt {
	case Float64:
		return strconv.ParseFloat(s, 64)
	case Uint16:
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("invalid uint16 value: %s", s)
		}
		if i < 0 || i > 65535 {
			return nil, fmt.Errorf("uint16 value out of range: %s", s)
		}
		return uint16(i), nil
	default:
		return nil, fmt.Errorf("unsupported data type: %s", dt)
	}
}

type Client struct {
	ctx     context.Context
	ns      uint16
	opc     *client.Client
	types   map[string]DataType
	write   chan *ua.WriteRequest
	cancel  context.CancelFunc
	mapping map[string]string
}

func NewClient(params map[string]string, mapping map[string]string) (c *Client, err error) {
	address := params["address"]
	if address == "" {
		return nil, errors.New("address is required")
	}
	namespaceStr, ok := params["namespace"]
	if !ok {
		return nil, errors.New("namespace is required")
	}
	namespace, err := strconv.Atoi(namespaceStr)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace index: %w", err)
	}
	username := params["username"]
	if username == "" {
		return nil, errors.New("user is required")
	}
	password := params["password"]
	if password == "" {
		return nil, errors.New("password is required")
	}

	c = &Client{
		ns:      uint16(namespace),
		write:   make(chan *ua.WriteRequest, 1),
		types:   make(map[string]DataType),
		mapping: mapping,
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.opc, err = client.Dial(
		c.ctx,
		address,
		client.WithClientCertificatePaths("opcua.crt", "opcua.key"),
		client.WithInsecureSkipVerify(),
		client.WithUserNameIdentity(username, password),
		client.WithApplicationName("dashy"),
		client.WithSecurityPolicyURI(ua.SecurityPolicyURIBasic256Sha256, ua.MessageSecurityModeSignAndEncrypt),
	)
	if err != nil {
		return nil, fmt.Errorf("error opening client connection: %w", err)
	}

	req := &ua.ReadRequest{
		NodesToRead: []ua.ReadValueID{},
	}
	for _, v := range mapping {
		req.NodesToRead = append(req.NodesToRead, ua.ReadValueID{
			NodeID:      ua.NodeIDString{NamespaceIndex: c.ns, ID: v},
			AttributeID: ua.AttributeIDDataType,
		})
	}
	resp, err := c.opc.Read(c.ctx, req)
	if err != nil {
		fmt.Printf("Error reading ServerStatus. %s\n", err.Error())
		return
	}
	for i, v := range resp.Results {
		c.types[req.NodesToRead[i].NodeID.(ua.NodeIDString).ID] = DataType(v.Value.(ua.NodeIDNumeric).ID)
	}
	for k, dt := range c.types {
		log.Info().Str("node", k).Str("type", dt.String()).Msg("node type resolved")
	}
	go func() {
		defer func() {
			log.Info().Msg("OPC UA client write loop stopped")
		}()
		log.Info().Str("address", address).Msg("OPC UA client write loop started")
		for wreq := range c.write {
			// send write request to server

			_, err := c.opc.Write(c.ctx, wreq)
			if err != nil {
				log.Error().Err(err).Msg("error writing values to OPC UA server")
				continue
			}

		}
	}()
	return c, nil
}

func (c *Client) Close() error {
	close(c.write)
	c.cancel()
	ctx := context.Background()
	err := c.opc.Close(ctx)
	if err != nil {
		c.opc.Abort(ctx)
		return err
	}
	return nil
}

func (c *Client) Send(values value.Container) error {
	wreq := &ua.WriteRequest{
		NodesToWrite: make([]ua.WriteValue, 0, len(values)),
	}
	for k, v := range values.Map(c.mapping) {
		dt, ok := c.types[k]
		if !ok {
			log.Error().Str("node", k).Msg("unknown node type, skipping write")
			continue
		}
		val, err := dt.Parse(v.Value)
		if err != nil {
			log.Error().Err(err).Str("node", k).Msg("error parsing value, skipping write")
			continue
		}
		log.Info().Str("node", k).Str("value", fmt.Sprintf("%v", val)).Msg("writing value to OPC UA server")
		wreq.NodesToWrite = append(wreq.NodesToWrite, ua.WriteValue{
			NodeID:      ua.NodeIDString{NamespaceIndex: c.ns, ID: k},
			AttributeID: ua.AttributeIDValue,
			Value:       ua.NewDataValue(val, ua.Good, v.Timestamp, 0, time.Now(), 0),
		})
	}
	if len(wreq.NodesToWrite) == 0 {
		return nil // nothing to write
	}
	c.write <- wreq
	return nil
}
