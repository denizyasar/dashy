// Copyright 2021 Converter Systems LLC. All rights reserved.

package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/NorthlandPowerEurope/dashy/opcua/client"
	"github.com/NorthlandPowerEurope/dashy/opcua/ua"
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
		return strconv.ParseUint(s, 10, 16)
	default:
		return nil, fmt.Errorf("unsupported data type: %s", dt)
	}
}

func main() {
	if err := ensurePKI(); err != nil {
		log.Fatalf("Error ensuring PKI: %s", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())

	// open a connection to testserver running locally.
	ch, err := client.Dial(
		ctx,
		"opc.tcp://localhost:4840",
		client.WithClientCertificatePaths("./pki/client.crt", "./pki/client.key"),
		client.WithInsecureSkipVerify(), // skips verification of server certificate
		client.WithUserNameIdentity("user", "user"),
		client.WithApplicationName("TestClient"),
		client.WithSecurityPolicyURI(ua.SecurityPolicyURIBasic256Sha256, ua.MessageSecurityModeSignAndEncrypt),
	)
	if err != nil {
		fmt.Printf("Error opening client connection. %s\n", err.Error())
		return
	}

	// prepare read request
	req := &ua.ReadRequest{
		NodesToRead: []ua.ReadValueID{
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.Windspeed"},
				AttributeID: ua.AttributeIDDataType,
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.WindDirection"},
				AttributeID: ua.AttributeIDDataType,
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.CloudLayer1"},
				AttributeID: ua.AttributeIDDataType,
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.CloudHeight1"},
				AttributeID: ua.AttributeIDDataType,
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.CloudLayer2"},
				AttributeID: ua.AttributeIDDataType,
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.CloudHeight2"},
				AttributeID: ua.AttributeIDDataType,
			},
		},
	}
	resp, err := ch.Read(ctx, req)
	if err != nil {
		fmt.Printf("Error reading ServerStatus. %s\n", err.Error())
		return
	}
	for i, v := range resp.Results {
		k := req.NodesToRead[i].NodeID.(ua.NodeIDString).ID
		fmt.Printf("%s DataType: %s\n", k, DataType(v.Value.(ua.NodeIDNumeric).ID))
	}
	wreq := &ua.WriteRequest{
		NodesToWrite: []ua.WriteValue{
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.Windspeed"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(float64(2.1), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.WindDirection"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(float64(180), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.CloudLayer1"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue("None", ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.CloudLayer2"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue("None", ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.CloudHeight1"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(uint16(0), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.CloudHeight2"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(uint16(0), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.Visibility"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(float64(10.0), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.QNH"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(float64(1013.25), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.SigWaveHeight"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(float64(1.5), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.MaxWaveHeight"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(float64(2.0), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.MeanWavePeriod"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(float64(5.0), ua.Good, time.Now(), 0, time.Now(), 0),
			},
			{
				NodeID:      ua.NodeIDString{NamespaceIndex: 2, ID: "Park.N1.WaveDirection"},
				AttributeID: ua.AttributeIDValue,
				Value:       ua.NewDataValue(float64(270), ua.Good, time.Now(), 0, time.Now(), 0),
			},
		},
	}
	_, err = ch.Write(ctx, wreq)
	if err != nil {
		fmt.Printf("Error writing int. %s\n", err.Error())
	}

	// wait for signal (this conflicts with debugger currently)
	log.Println("Press Ctrl-C to exit...")
	waitForSignal()

	log.Println("Closing client...")
	cancel()

	ctx = context.Background()
	err = ch.Close(ctx)
	if err != nil {
		ch.Abort(ctx)
		return
	}
	log.Println("Client closed.")

}

func waitForSignal() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}

func createNewCertificate(appName, certFile, keyFile string) error {

	// create a keypair.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return ua.BadCertificateInvalid
	}

	// get local hostname.
	host, _ := os.Hostname()

	// get local ip address.
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return ua.BadCertificateInvalid
	}
	conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	// create a certificate.
	applicationURI, _ := url.Parse(fmt.Sprintf("urn:%s:%s", host, appName))
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	subjectKeyHash := sha1.New()
	subjectKeyHash.Write(key.PublicKey.N.Bytes())
	subjectKeyId := subjectKeyHash.Sum(nil)
	oidDC := asn1.ObjectIdentifier([]int{0, 9, 2342, 19200300, 100, 1, 25})

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: appName, ExtraNames: []pkix.AttributeTypeAndValue{{Type: oidDC, Value: host}}},
		SubjectKeyId:          subjectKeyId,
		AuthorityKeyId:        subjectKeyId,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{host, "localhost"},
		IPAddresses:           []net.IP{localAddr.IP, []byte{127, 0, 0, 1}},
		URIs:                  []*url.URL{applicationURI},
	}

	rawcrt, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return ua.BadCertificateInvalid
	}

	if f, err := os.Create(certFile); err == nil {
		block := &pem.Block{Type: "CERTIFICATE", Bytes: rawcrt}
		if err := pem.Encode(f, block); err != nil {
			f.Close()
			return err
		}
		f.Close()
	} else {
		return err
	}

	if f, err := os.Create(keyFile); err == nil {
		block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
		if err := pem.Encode(f, block); err != nil {
			f.Close()
			return err
		}
		f.Close()
	} else {
		return err
	}

	return nil
}

func ensurePKI() error {

	// check if ./pki already exists
	if _, err := os.Stat("./pki"); !os.IsNotExist(err) {
		return nil
	}

	// make a pki directory, if not exist
	if err := os.MkdirAll("./pki", os.ModeDir|0755); err != nil {
		return err
	}

	// create a server cert in ./pki/server.crt
	if err := createNewCertificate("testclient", "./pki/client.crt", "./pki/client.key"); err != nil {
		return err
	}

	return nil
}
