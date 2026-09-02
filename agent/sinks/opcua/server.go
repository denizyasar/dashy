package opcua

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/NorthlandPowerEurope/dashy/agent/auth"
	"github.com/NorthlandPowerEurope/dashy/opcua/server"
	"github.com/NorthlandPowerEurope/dashy/opcua/ua"
)

type Config struct {
	Port    uint16
	Info    Info
	NodeSet []byte // nodeset.xml content
	Users   *auth.Provider
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

type Server struct {
	cfg Config
	srv *server.Server
}

func NewServer(cfg Config) (*Server, error) {

	// create the endpoint url from hostname and port
	host, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	port, err := cfg.port()
	if err != nil {
		return nil, fmt.Errorf("failed to find free port: %w", err)
	}
	endpointURL := fmt.Sprintf("opc.tcp://%s:%d", host, port)

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
		filepath.Join("opcua.crt"),
		filepath.Join("opcua.key"),
		endpointURL,
		server.WithBuildInfo(
			ua.BuildInfo{
				ProductURI:       cfg.Info.repository(),
				ManufacturerName: cfg.Info.manufacturer(),
				ProductName:      cfg.Info.name(),
				SoftwareVersion:  cfg.Info.version(),
			}),
		server.WithAuthenticateUserNameIdentityFunc(func(userIdentity ua.UserNameIdentity, applicationURI string, endpointURL string) error {
			valid := cfg.Users.IsValid(userIdentity.UserName, userIdentity.Password)
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
		return nil, err
	}

	nm := srv.NamespaceManager()
	if err := nm.LoadNodeSetFromBuffer(cfg.NodeSet); err != nil {
		return nil, fmt.Errorf("failed to load nodeset: %w", err)
	}

	go func() {
		defer func() {
			log.Println("OPC UA server stopped")
		}()
		log.Printf("Starting server '%s' at '%s'\n", srv.LocalDescription().ApplicationName.Text, srv.EndpointURL())
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Error starting server: %s\n", err)
		}
	}()

	return &Server{
		cfg: cfg,
		srv: srv,
	}, nil
}

func (s *Server) Close() error {
	return s.srv.Close()
}
