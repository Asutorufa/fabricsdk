/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package grpcclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type GRPCClient struct {
	// TLS configuration used by the grpc.ClientConn
	tlsConfig *tls.Config
	// Options for setting up new connections
	dialOpts []grpc.DialOption
	// Duration for which to block while established a new connection
	timeout time.Duration
	// Maximum message size the client can receive
	maxRecvMsgSize int
	// Maximum message size the client can send
	maxSendMsgSize int
}

// NewGRPCClient creates a new implementation of GRPCClient given an address
// and client configuration
func NewGRPCClient(config *ClientConfig) (*GRPCClient, error) {
	client := &GRPCClient{}

	// parse secure options
	err := client.parseSecureOptions(config.SecOpts)
	if err != nil {
		return client, err
	}

	// keepalive options

	kap := keepalive.ClientParameters{
		Time:                config.KaOpts.ClientInterval,
		Timeout:             config.KaOpts.ClientTimeout,
		PermitWithoutStream: true,
	}
	// set keepalive
	client.dialOpts = append(client.dialOpts, grpc.WithKeepaliveParams(kap))
	// Unless asynchronous connect is set, make connection establishment blocking.
	if !config.AsyncConnect {
		client.dialOpts = append(client.dialOpts, grpc.WithBlock())
		client.dialOpts = append(client.dialOpts, grpc.FailOnNonTempDialError(true))
	}
	client.timeout = config.Timeout
	// set send/recv message size to package defaults
	client.maxRecvMsgSize = MaxRecvMsgSize
	client.maxSendMsgSize = MaxSendMsgSize

	return client, nil
}

func (client *GRPCClient) parseSecureOptions(opts SecureOptions) error {
	// if TLS is not enabled, return
	if !opts.UseTLS {
		return nil
	}

	client.tlsConfig = &tls.Config{
		VerifyPeerCertificate: opts.VerifyCertificate,
		MinVersion:            tls.VersionTLS12,
		InsecureSkipVerify:    opts.InsecureSkipVerify,
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		log.Println("get system cert pool failed:", err)
		certPool = x509.NewCertPool()
	}
	client.tlsConfig.RootCAs = certPool

	if len(opts.ServerRootCAs) > 0 {
		for _, certBytes := range opts.ServerRootCAs {
			err := AddPemToCertPool(certBytes, client.tlsConfig.RootCAs)
			if err != nil {
				return fmt.Errorf("error adding root certificate: %w", err)
			}
		}
	}

	if opts.RequireClientCert {
		// make sure we have both Key and Certificate
		if opts.Key == nil || opts.Certificate == nil {
			return errors.New("both Key and Certificate are required when using mutual TLS")
		}

		cert, err := tls.X509KeyPair(opts.Certificate, opts.Key)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %w", err)
		}
		client.tlsConfig.Certificates = append(client.tlsConfig.Certificates, cert)
	}

	if opts.TimeShift > 0 {
		client.tlsConfig.Time = func() time.Time { return time.Now().Add((-1) * opts.TimeShift) }
	}

	return nil
}

// Certificate returns the tls.Certificate used to make TLS connections
// when client certificates are required by the server
func (client *GRPCClient) Certificate() tls.Certificate {
	cert := tls.Certificate{}
	if client.tlsConfig != nil && len(client.tlsConfig.Certificates) > 0 {
		cert = client.tlsConfig.Certificates[0]
	}
	return cert
}

// TLSEnabled is a flag indicating whether to use TLS for client
// connections
func (client *GRPCClient) TLSEnabled() bool {
	return client.tlsConfig != nil
}

// MutualTLSRequired is a flag indicating whether the client
// must send a certificate when making TLS connections
func (client *GRPCClient) MutualTLSRequired() bool {
	return client.tlsConfig != nil &&
		len(client.tlsConfig.Certificates) > 0
}

// SetMaxRecvMsgSize sets the maximum message size the client can receive
func (client *GRPCClient) SetMaxRecvMsgSize(size int) {
	client.maxRecvMsgSize = size
}

// SetMaxSendMsgSize sets the maximum message size the client can send
func (client *GRPCClient) SetMaxSendMsgSize(size int) {
	client.maxSendMsgSize = size
}

// SetServerRootCAs sets the list of authorities used to verify server
// certificates based on a list of PEM-encoded X509 certificate authorities
func (client *GRPCClient) SetServerRootCAs(serverRoots [][]byte) error {
	// NOTE: if no serverRoots are specified, the current cert pool will be
	// replaced with an empty one
	certPool := x509.NewCertPool()
	for _, root := range serverRoots {
		err := AddPemToCertPool(root, certPool)
		if err != nil {
			return fmt.Errorf("error adding root certificate: %w", err)
		}
	}
	client.tlsConfig.RootCAs = certPool
	return nil
}

type TLSOption func(tlsConfig *tls.Config)

func ServerNameOverride(name string) TLSOption {
	return func(tlsConfig *tls.Config) {
		tlsConfig.ServerName = name
	}
}

func CertPoolOverride(pool *x509.CertPool) TLSOption {
	return func(tlsConfig *tls.Config) {
		tlsConfig.RootCAs = pool
	}
}

// NewConnection returns a grpc.ClientConn for the target address and
// overrides the server name used to verify the hostname on the
// certificate returned by a server when using TLS
func (client *GRPCClient) NewConnection(address string, tlsOptions ...TLSOption) (*grpc.ClientConn, error) {
	var dialOpts []grpc.DialOption
	dialOpts = append(dialOpts, client.dialOpts...)

	// set transport credentials and max send/recv message sizes
	// immediately before creating a connection in order to allow
	// SetServerRootCAs / SetMaxRecvMsgSize / SetMaxSendMsgSize
	//  to take effect on a per connection basis
	if client.tlsConfig != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(
			&DynamicClientCredentials{TLSConfig: client.tlsConfig, TLSOptions: tlsOptions},
		))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}

	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(client.maxRecvMsgSize),
		grpc.MaxCallSendMsgSize(client.maxSendMsgSize),
	))

	ctx, cancel := context.WithTimeout(context.Background(), client.timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create new connection: %w", err)
	}
	return conn, nil
}
