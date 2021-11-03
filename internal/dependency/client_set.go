package dependency

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	consulapi "github.com/hashicorp/consul/api"
	rootcerts "github.com/hashicorp/go-rootcerts"
	vaultapi "github.com/hashicorp/vault/api"
)

// ClientSet is a collection of clients that dependencies use to communicate
// with remote services like Consul or Vault.
type ClientSet struct {
	sync.RWMutex

	vault  *vaultClient
	consul *consulClient
}

// consulClient is a wrapper around a real Consul API client.
type consulClient struct {
	client     *consulapi.Client
	httpClient *http.Client
}

// vaultClient is a wrapper around a real Vault API client.
type vaultClient struct {
	client     *vaultapi.Client
	httpClient *http.Client
}

// CreateClientInput is used as input to the CreateClient functions.
type CreateClientInput struct {
	Address   string
	Namespace string
	Token     string
	// vault only
	UnwrapToken bool
	// consul only
	AuthEnabled  bool
	AuthUsername string
	AuthPassword string
	// Transport/TLS
	SSLEnabled bool
	SSLVerify  bool
	SSLCert    string
	SSLKey     string
	SSLCACert  string
	SSLCAPath  string
	ServerName string

	TransportDialKeepAlive       time.Duration
	TransportDialTimeout         time.Duration
	TransportDisableKeepAlives   bool
	TransportIdleConnTimeout     time.Duration
	TransportMaxIdleConns        int
	TransportMaxIdleConnsPerHost int
	TransportTLSHandshakeTimeout time.Duration

	// optional, principally for testing
	HttpClient *http.Client
}

// NewClientSet creates a new client set that is ready to accept clients.
func NewClientSet() *ClientSet {
	return &ClientSet{}
}

// CreateConsulClient creates a new Consul API client from the given input.
func (c *ClientSet) CreateConsulClient(i *CreateClientInput) error {
	consulConfig := consulapi.DefaultConfig()

	if i.Address != "" {
		consulConfig.Address = i.Address
	}

	if i.Namespace != "" {
		consulConfig.Namespace = i.Namespace
	}

	if i.Token != "" {
		consulConfig.Token = i.Token
	}

	if i.AuthEnabled {
		consulConfig.HttpAuth = &consulapi.HttpBasicAuth{
			Username: i.AuthUsername,
			Password: i.AuthPassword,
		}
	}

	// set/create our HTTP client
	if client, err := httpClient(i); err != nil {
		return err
	} else {
		consulConfig.HttpClient = client
	}

	// Setup the new transport
	if i.SSLEnabled {
		consulConfig.Scheme = "https"
	}

	// Create the API client
	client, err := consulapi.NewClient(consulConfig)
	if err != nil {
		return fmt.Errorf("client set: consul: %s", err)
	}

	if err := hasLeader(client, time.Minute); err != nil {
		return err
	}

	// Save the data on ourselves
	c.Lock()
	c.consul = &consulClient{
		client:     client,
		httpClient: consulConfig.HttpClient,
	}
	c.Unlock()

	return nil
}

func hasLeader(client *consulapi.Client, maxRetryWait time.Duration) error {
	// spin until Consul cluster has a leader
	retryTime := time.Second
	for {
		leader, err := client.Status().Leader()
		switch e := err.(type) {
		case net.Error:
			if !e.Temporary() {
				return e
			}
		default:
		}
		if leader != "" { // will contain the url of leader if good
			return nil
		}
		retryTime = retryTime * 2
		if retryTime > maxRetryWait {
			return fmt.Errorf("client set: no consul leader detected")
		}
		time.Sleep(retryTime)
	}
}

func (c *ClientSet) CreateVaultClient(i *CreateClientInput) error {
	vaultConfig := vaultapi.DefaultConfig()

	if i.Address != "" {
		vaultConfig.Address = i.Address
	}

	// set/create our HTTP client
	if client, err := httpClient(i); err != nil {
		return err
	} else {
		vaultConfig.HttpClient = client
	}

	// Create the client
	client, err := vaultapi.NewClient(vaultConfig)
	if err != nil {
		return fmt.Errorf("client set: vault: %s", err)
	}

	// Set the namespace if given.
	if i.Namespace != "" {
		client.SetNamespace(i.Namespace)
	}

	// Set the token if given
	if i.Token != "" {
		client.SetToken(i.Token)
	}

	// Check if we are unwrapping
	if i.UnwrapToken {
		secret, err := client.Logical().Unwrap(i.Token)
		if err != nil {
			return fmt.Errorf("client set: vault unwrap: %s", err)
		}

		if secret == nil {
			return fmt.Errorf("client set: vault unwrap: no secret")
		}

		if secret.Auth == nil {
			return fmt.Errorf("client set: vault unwrap: no secret auth")
		}

		if secret.Auth.ClientToken == "" {
			return fmt.Errorf("client set: vault unwrap: no token returned")
		}

		client.SetToken(secret.Auth.ClientToken)
	}

	// Save the data on ourselves
	c.Lock()
	c.vault = &vaultClient{
		client:     client,
		httpClient: vaultConfig.HttpClient,
	}
	c.Unlock()

	return nil
}

// Consul returns the Consul client for this set.
func (c *ClientSet) Consul() *consulapi.Client {
	c.RLock()
	defer c.RUnlock()
	if c == nil || c.consul == nil {
		return nil
	}
	return c.consul.client
}

// Vault returns the Vault client for this set.
func (c *ClientSet) Vault() *vaultapi.Client {
	c.RLock()
	defer c.RUnlock()
	if c == nil || c.vault == nil {
		return nil
	}
	return c.vault.client
}

// Stop closes all idle connections for any attached clients.
func (c *ClientSet) Stop() {
	c.Lock()
	defer c.Unlock()

	switch {
	case c.consul == nil:
	case c.consul.httpClient == nil:
	default:
		c.consul.httpClient.CloseIdleConnections()
	}

	switch {
	case c.vault == nil:
	case c.vault.httpClient == nil:
	default:
		c.vault.httpClient.CloseIdleConnections()
	}
}

// httpClient returns the http.Client to use with the API client.
// Returns the test one if given, otherwise creates one with default transport.
func httpClient(i *CreateClientInput) (client *http.Client, err error) {
	if i.HttpClient != nil {
		return i.HttpClient, nil
	}
	var transport *http.Transport
	if transport, err = newTransport(i); err == nil {
		client = &http.Client{
			Transport: transport,
		}
	}
	return client, err
}

func newTransport(i *CreateClientInput) (*http.Transport, error) {
	// This transport will attempt to keep connections open to the server.
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   i.TransportDialTimeout,
			KeepAlive: i.TransportDialKeepAlive,
		}).Dial,
		DisableKeepAlives:   i.TransportDisableKeepAlives,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        i.TransportMaxIdleConns,
		IdleConnTimeout:     i.TransportIdleConnTimeout,
		MaxIdleConnsPerHost: i.TransportMaxIdleConnsPerHost,
		TLSHandshakeTimeout: i.TransportTLSHandshakeTimeout,
	}

	// Configure SSL
	if i.SSLEnabled {

		var tlsConfig tls.Config

		// Custom certificate or certificate and key
		if i.SSLCert != "" && i.SSLKey != "" {
			cert, err := tls.LoadX509KeyPair(i.SSLCert, i.SSLKey)
			if err != nil {
				return nil, fmt.Errorf("client set: ssl: %s", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		} else if i.SSLCert != "" {
			cert, err := tls.LoadX509KeyPair(i.SSLCert, i.SSLCert)
			if err != nil {
				return nil, fmt.Errorf("client set: ssl: %s", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		// Custom CA certificate
		if i.SSLCACert != "" || i.SSLCAPath != "" {
			rootConfig := &rootcerts.Config{
				CAFile: i.SSLCACert,
				CAPath: i.SSLCAPath,
			}
			if err := rootcerts.ConfigureTLS(&tlsConfig, rootConfig); err != nil {
				return nil, fmt.Errorf("client set: configuring TLS failed: %s", err)
			}
		}

		// Construct all the certificates now
		tlsConfig.BuildNameToCertificate()

		// SSL verification
		if i.ServerName != "" {
			tlsConfig.ServerName = i.ServerName
			tlsConfig.InsecureSkipVerify = false
		}
		if !i.SSLVerify {
			tlsConfig.InsecureSkipVerify = true
		}

		// Save the TLS config on our transport
		transport.TLSClientConfig = &tlsConfig
	}
	return transport, nil
}
