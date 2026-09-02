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
	"log"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "embed"

	"github.com/NorthlandPowerEurope/dashy/opcua/server"
	"github.com/NorthlandPowerEurope/dashy/opcua/ua"
	"golang.org/x/crypto/bcrypt"
)

//go:embed nodeset_n1.xml
var nodeSet []byte

type Config struct {
	Port    uint16
	Info    Info
	NodeSet []byte // nodeset.xml content
	Auth    map[string]string
}

type Info struct {
	ID           string
	Name         string
	Version      string
	Repository   string
	Manufacturer string
}

type User struct {
	ua.UserNameIdentity
}

func (info Info) id() string {
	if info.ID == "" || strings.Contains(info.ID, " ") {
		return "opcua"
	}
	return info.ID
}

func (info Info) name() string {
	if info.Name == "" {
		return "OPC UA"
	}
	return info.Name
}

func (info Info) version() string {
	if info.Version == "" {
		return "1.0.0"
	}
	return info.Version
}

func (info Info) repository() string {
	if info.Repository == "" {
		return "http://github.com/malivvan/opcua"
	}
	return info.Repository
}

func (info Info) manufacturer() string {
	if info.Manufacturer == "" {
		return "malivvan"
	}
	return info.Manufacturer
}

func (cfg Config) auth() ([]ua.UserNameIdentity, error) {
	auth := []ua.UserNameIdentity{}
	for username, password := range cfg.Auth {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 8)
		if err != nil {
			return nil, err
		}
		auth = append(auth, ua.UserNameIdentity{
			UserName: username,
			Password: string(hash),
		})
	}
	return auth, nil
}

func (cfg Config) port() (uint16, error) {
	if cfg.Port > 0 {
		return cfg.Port, nil
	}
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	if addr.Port <= 0 || addr.Port > 65535 {
		return 0, fmt.Errorf("invalid port %d", addr.Port)
	}
	return uint16(addr.Port), nil
}

func main() {
	if err := Run(Config{
		Port: 4840,
		Info: Info{
			ID:           "dashyua",
			Name:         "Dashy OPC UA Server",
			Version:      "0.3.0",
			Repository:   "http://github.com/NorthlandPowerEurope/dashy/opcua",
			Manufacturer: "Northland Power Europe",
		},
		Auth: map[string]string{
			"admin": "admin",
			"user":  "user",
		},
		NodeSet: nodeSet,
	}); err != nil {
		log.Fatalf("Failed to run OPC UA server: %v", err)

	}
}

func Run(cfg Config) error {

	// create directory with certificate and key, if not found.
	if err := ensurePKI(); err != nil {
		return err
	}

	// create the endpoint url from hostname and port
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	port, err := cfg.port()
	if err != nil {
		return fmt.Errorf("failed to find free port: %w", err)
	}
	endpointURL := fmt.Sprintf("opc.tcp://%s:%d", host, port)

	auth, err := cfg.auth()
	if err != nil {
		return fmt.Errorf("failed to configure authentication: %w", err)
	}

	// create server
	srv, err := server.New(
		ua.ApplicationDescription{
			ApplicationURI: fmt.Sprintf("urn:%s:%s", host, cfg.Info.id()),
			ProductURI:     cfg.Info.repository(),
			ApplicationName: ua.LocalizedText{
				Text:   fmt.Sprintf("%s@%s", cfg.Info.id(), host),
				Locale: "en",
			},
			ApplicationType: ua.ApplicationTypeServer,
			DiscoveryURLs:   []string{endpointURL},
		},
		"./pki/server.crt",
		"./pki/server.key",
		endpointURL,
		server.WithBuildInfo(
			ua.BuildInfo{
				ProductURI:       cfg.Info.repository(),
				ManufacturerName: cfg.Info.manufacturer(),
				ProductName:      cfg.Info.name(),
				SoftwareVersion:  cfg.Info.version(),
			}),
		server.WithAuthenticateUserNameIdentityFunc(func(userIdentity ua.UserNameIdentity, applicationURI string, endpointURL string) error {
			valid := false
			for _, user := range auth {
				if user.UserName == userIdentity.UserName {
					if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(userIdentity.Password)); err == nil {
						valid = true
						break
					}
				}
			}
			if !valid {
				return ua.BadUserAccessDenied
			}
			log.Printf("Login %s from %s\n", userIdentity.UserName, applicationURI)
			return nil
		}),

		server.WithInsecureSkipVerify(),       // do not verify client certificate
		server.WithMaxWorkerThreads(1),        // TODO: maybe raise to core count
		server.WithMaxSessionCount(100),       // 100 should be enough
		server.WithMaxSubscriptionCount(1000), // 1000 should be enough
		server.WithServerDiagnostics(true),
	)
	if err != nil {
		return err
	}

	// load nodeset
	//    <UAVariable DataType="Double" NodeId="ns=1;s=Demo.Static.Scalar.Double" BrowseName="1:Double" UserAccessLevel="3" AccessLevel="3">
	//    <DisplayName>Double</DisplayName>
	//    <References>
	//        <Reference ReferenceType="HasTypeDefinition">i=63</Reference>
	//        <Reference ReferenceType="Organizes" IsForward="false">ns=1;s=Demo.Static.Scalar</Reference>
	//    </References>
	//    <Value>
	//       <uax:Double>3.14</uax:Double>
	//    </Value>
	//		bool, int8, uint8, int16, uint16, int32, uint32
	//	int64, uint64, float32, float64, string
	//	time.Time, uuid.UUID, ByteString, XmlElement
	//	NodeId, ExpandedNodeId, StatusCode, QualifiedName
	//	LocalizedText, DataValue, Variant

	//</UAVariable>
	nm := srv.NamespaceManager()

	if err := nm.LoadNodeSetFromBuffer(cfg.NodeSet); err != nil {
		return fmt.Errorf("failed to load nodeset: %w", err)
	}

	go func() {
		// wait for signal
		log.Println("Press Ctrl-C to exit...")
		waitForSignal()

		log.Println("Stopping server...")
		srv.Close()
	}()

	// start server
	log.Printf("Starting server '%s' at '%s'\n", srv.LocalDescription().ApplicationName.Text, srv.EndpointURL())
	return srv.ListenAndServe()
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
	if err := createNewCertificate("testserver", "./pki/server.crt", "./pki/server.key"); err != nil {
		return err
	}

	return nil
}

// getNextEventID gets next random eventID.
func getNextEventID() ua.ByteString {
	var nonce = make([]byte, 16)
	rand.Read(nonce)
	return ua.ByteString(nonce)
}
