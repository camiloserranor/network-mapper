// Package gnmi provides a gNMI client for querying TOR switches.
package gnmi

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TLSOptions configures TLS behavior for gNMI connections.
type TLSOptions struct {
	SkipVerify bool
	TOFU       bool
	CertDir    string
	CACert     string
	ClientCert string
	ClientKey  string
}

const tofuProbeTimeout = 10 * time.Second

// BuildTLSConfig creates a *tls.Config for the given switch address.
func BuildTLSConfig(address string, opts TLSOptions) (*tls.Config, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}

	tlsCfg := &tls.Config{}

	// Client certificate (mTLS)
	if opts.ClientCert != "" && opts.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(opts.ClientCert, opts.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("loading client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// Explicit CA certificate
	if opts.CACert != "" {
		pool, err := loadCACertPool(opts.CACert)
		if err != nil {
			return nil, err
		}
		tlsCfg.RootCAs = pool
		tlsCfg.ServerName = host
		return tlsCfg, nil
	}

	// TOFU: trust-on-first-use with cert caching
	if opts.TOFU {
		certFile := certPath(host, opts.CertDir)

		// Try loading cached cert
		pool, serverName, err := loadCachedCertPool(certFile)
		if err == nil {
			log.Printf("TOFU: loaded cached cert for %s", host)
			tlsCfg.RootCAs = pool
			tlsCfg.ServerName = serverName
			return tlsCfg, nil
		}

		// Fetch and cache
		log.Printf("TOFU: fetching cert from %s", address)
		serverCert, err := fetchServerCert(address)
		if err != nil {
			return nil, fmt.Errorf("TOFU cert fetch for %s: %w", address, err)
		}

		fp := certFingerprint(serverCert)
		log.Printf("TOFU: trusted cert from %s (SHA-256: %s)", address, fp)

		if err := saveCertPEM(serverCert, certFile); err != nil {
			log.Printf("WARN: could not cache cert for %s: %v", host, err)
		}

		pool = x509.NewCertPool()
		pool.AddCert(serverCert)
		tlsCfg.RootCAs = pool
		tlsCfg.ServerName = certServerName(serverCert, host)
		return tlsCfg, nil
	}

	// Skip verify (insecure)
	if opts.SkipVerify {
		tlsCfg.InsecureSkipVerify = true
		return tlsCfg, nil
	}

	// Default: system CA pool
	tlsCfg.ServerName = host
	return tlsCfg, nil
}

func fetchServerCert(address string) (*x509.Certificate, error) {
	dialer := &net.Dialer{Timeout: tofuProbeTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", address, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil, fmt.Errorf("TLS probe to %s: %w", address, err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates from %s", address)
	}
	return certs[0], nil
}

func certPath(host, certDir string) string {
	safe := strings.ReplaceAll(host, ":", "_")
	return filepath.Join(certDir, safe+".pem")
}

func loadCACertPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert %s: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("no valid certs in %s", path)
	}
	return pool, nil
}

func loadCachedCertPool(path string) (*x509.CertPool, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, "", fmt.Errorf("no PEM block in %s", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("parsing cached cert: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return pool, certServerName(cert, ""), nil
}

func saveCertPEM(cert *x509.Certificate, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating cert dir: %w", err)
	}
	block := &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
	data := pem.EncodeToMemory(block)

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func certFingerprint(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	parts := make([]string, len(hash))
	for i, b := range hash {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":")
}

func certServerName(cert *x509.Certificate, fallback string) string {
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	if len(cert.IPAddresses) > 0 {
		return cert.IPAddresses[0].String()
	}
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName
	}
	return fallback
}
