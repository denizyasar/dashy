package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NorthlandPowerEurope/dashy/agent/auth"
	"github.com/NorthlandPowerEurope/dashy/agent/sinks"
	"github.com/NorthlandPowerEurope/dashy/agent/sinks/opcua"
	"github.com/NorthlandPowerEurope/dashy/agent/sinks/value"
	"github.com/NorthlandPowerEurope/dashy/agent/sources"
	"github.com/NorthlandPowerEurope/dashy/opcua/ua"
	"github.com/malivvan/servicekit"
	"github.com/malivvan/servicekit/log"
)

type Config struct {
	OPCUA    *OPCUAConfig    `json:"opcua"`
	Sinks    []SinkConfig    `json:"sinks"`
	Routines []RoutineConfig `json:"routines"`
	Logging  log.Config      `json:"logging"`
}

type OPCUAConfig struct {
	Port    int    `json:"port"`
	Users   string `json:"users"`
	NodeSet string `json:"nodeset"`
}

type RoutineConfig struct {
	Name            string       `json:"name"`
	Source          SourceConfig `json:"source"`
	Sinks           []string     `json:"sinks"`
	IntervalSeconds int          `json:"interval_seconds"`
}
type SinkConfig struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Params  map[string]string `json:"params"`
	Mapping map[string]string `json:"mapping"`
}

type SourceConfig struct {
	Type   string            `json:"type"`
	Params map[string]string `json:"params"`
}

type DestinationConfig struct {
	Address string `json:"address"`
	Token   string `json:"token"`
}

type Routine struct {
	name      string
	running   atomic.Bool
	waitgroup sync.WaitGroup
	source    sources.Source
	senders   []sinks.Sender
	interval  time.Duration
}

func (r *Routine) run() {
	defer r.waitgroup.Done()

	nextRun := time.Now().Truncate(r.interval).Add(r.interval)
	for r.running.Load() {

		// wait until next run, stop early for service termination if required
		log.Info().
			Str("routine", r.name).
			Time("next_run", nextRun).
			Msg("waiting for next run")
		for r.running.Load() {
			waitTime := time.Until(nextRun)
			if waitTime > time.Second {
				time.Sleep(time.Second)
			} else {
				time.Sleep(waitTime)
				break
			}
		}
		log.Info().
			Str("routine", r.name).
			Msg("collecting data")

		// schedule next interval
		nextRun = time.Now().Truncate(r.interval).Add(r.interval)

		// get values
		values, err := r.source.Values()
		if err != nil {
			log.Error().Err(err).Msg("failed to retrieve values from source")
			continue
		}

		container := value.New()
		for k, v := range values {
			container.Add(k, v.Timestamp, v.Value)

		}

		// send values
		for _, sender := range r.senders {
			err = sender.Send(container)
			if err != nil {
				log.Error().Err(err).Msg("failed to send values to sink")
				continue
			}
		}
	}
}

func (r *Routine) Stop() {
	r.running.Store(false)
	r.waitgroup.Wait()
	r.source.Close()
}

type Service struct {
	opc      *opcua.Server
	routines []Routine
}

func (s *Service) Start(workdir servicekit.Workdir) error {

	config := &Config{}
	err := servicekit.Configure(workdir.Path("config.json"), config, &Config{
		Routines: []RoutineConfig{},
		Logging: log.Config{
			Level:    "info",
			Compress: true,
		},
	})
	if err != nil {
		return err
	}

	err = log.Start(workdir.Path("log", "dashy-agent.log"), &config.Logging)
	if err != nil {
		return err
	}
	log.Info().Str("name", "dashy-agent").Msg("starting service")

	if err := os.Chdir(workdir.Path()); err != nil {
		return fmt.Errorf("failed to change working directory: %w", err)
	}
	if err := ensureTLS(); err != nil {
		return fmt.Errorf("error creating tls server certificates: %w", err)
	}

	if config.OPCUA != nil {
		users, err := auth.New("opcua.json", ";f>3pVk`cp:@Z-Gw")
		if err != nil {
			return fmt.Errorf("failed to load users: %w", err)
		}
		nodeSet, err := os.ReadFile("opcua.xml")
		if err != nil {
			return fmt.Errorf("failed to load nodeset: %w", err)
		}
		s.opc, err = opcua.NewServer(opcua.Config{
			Port: 4840,
			Info: opcua.Info{
				ID:           "dashyua",
				Name:         "Dashy OPC UA Server",
				Version:      "2.0.0",
				Repository:   "http://github.com/denizyasar/dashy/opcua",
				Manufacturer: "Northland Power Europe",
			},
			Users:   users,
			NodeSet: nodeSet,
		})
		if err != nil {
			return fmt.Errorf("failed to start OPC UA server: %w", err)
		}
		log.Info().Str("name", "dashy-agent").Msg("OPC UA server started")
	}

	senderMap := make(map[string]sinks.Sender)
	for _, sinkConfig := range config.Sinks {
		log.Info().Str("sink", sinkConfig.Name).Msg("configuring sink")
		sender, err := sinks.New(sinkConfig.Type, sinkConfig.Params, sinkConfig.Mapping)
		if err != nil {
			log.Error().Err(err).Msg("failed to configure sink")
			return err
		}
		senderMap[sinkConfig.Name] = sender
		log.Info().Str("sink", sinkConfig.Name).Msg("sink configured successfully")
	}

	for _, routineConfig := range config.Routines {
		log.Info().Str("routine", routineConfig.Name).Msg("starting routine")

		// prepare source
		source, err := sources.Open(routineConfig.Source.Type, routineConfig.Source.Params)
		if err != nil {
			log.Error().Err(err).Msg("failed to configure source")
			return err
		}

		var senders []sinks.Sender
		for _, sinkName := range routineConfig.Sinks {
			sender, exists := senderMap[sinkName]
			if !exists {
				log.Error().Str("sink", sinkName).Msg("sink not found")
				return fmt.Errorf("sink %s not found", sinkName)
			}
			senders = append(senders, sender)
		}

		// create and run routine
		routine := &Routine{
			name:     routineConfig.Name,
			source:   source,
			senders:  senders,
			interval: time.Duration(routineConfig.IntervalSeconds) * time.Second,
		}
		routine.running.Store(true)
		routine.waitgroup.Add(1)
		go routine.run()

		log.Info().Str("routine", routineConfig.Name).Msg("routine started")
	}

	log.Info().Str("name", "dashy-agent").Msg("service started")
	return nil
}

func (s *Service) Stop() error {
	log.Info().Str("name", "dashy-agent").Msg("stopping service")
	for _, routine := range s.routines {
		log.Info().Str("routine", routine.name).Msg("stopping routine")
		routine.Stop()
		log.Info().Str("routine", routine.name).Msg("stopped routine")
	}
	if s.opc != nil {
		log.Info().Msg("stopping OPC UA server")
		if err := s.opc.Close(); err != nil {
			log.Error().Err(err).Msg("failed to stop OPC UA server")
		} else {
			log.Info().Msg("OPC UA server stopped")
		}
	}
	log.Info().Str("name", "dashy-agent").Msg("stopped service")
	return nil
}

func main() {
	servicekit.Wrap(servicekit.Config{
		Name:        "dashy-agent",
		Version:     "2.0.0",
		Description: "Agent sending values to Dashboard for Operational Values",
	}, new(Service))
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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
func ensureTLS() error {
	if fileExists("opcua.crt") && fileExists("opcua.key") {
		return nil
	}
	if err := createNewCertificate("dashy", "opcua.crt", "opcua.key"); err != nil {
		return err
	}

	return nil
}
