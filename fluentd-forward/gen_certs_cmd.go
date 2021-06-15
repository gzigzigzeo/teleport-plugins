/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net"
	"path"
	"time"

	"github.com/gravitational/trace"
)

type GenCertsCmd struct {
	// Out path and file prefix to put certificates into
	Out string `arg:"true" help:"Output directory" type:"existingdir" required:"true"`

	// Pwd key passphrase
	Pwd string `arg:"true" help:"Passphrase" required:"true"`

	// Certificate TTL
	TTL time.Duration `help:"Certificate TTL" required:"true" default:"87600h"`

	// DNSNames is a DNS subjectAltNames for server cert
	DNSNames []string `help:"Certificate SAN hosts" default:"localhost"`

	// HostNames is an IP subjectAltNames for server cert
	IP []string `help:"Certificate SAN IPs"`

	// Length is RSA key length
	Length int `help:"Key length" enum:"1024,2048,4096" default:"2048"`

	// CN certificate common name
	CN string `help:"Common name for server cert" default:"localhost"`
}

var (
	// maxBigInt is a reader for serial number random
	maxBigInt *big.Int = new(big.Int).Lsh(big.NewInt(1), 128)
)

// Run runs the generator
func (c *GenCertsCmd) Run() error {
	entity := pkix.Name{
		CommonName: c.CN,
		Country:    []string{"US"},
	}

	// CA CSR
	sn, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return trace.Wrap(err)
	}

	notBefore := time.Now()
	notAfter := time.Now().Add(c.TTL)

	caCert := &x509.Certificate{
		SerialNumber:          sn,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		MaxPathLenZero:        true,
		KeyUsage:              x509.KeyUsageCRLSign | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// Client CSR
	sn, err = rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return trace.Wrap(err)
	}

	clientCert := &x509.Certificate{
		SerialNumber: sn,
		Subject:      entity,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	sn, err = rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return trace.Wrap(err)
	}

	// Server CSR
	serverCert := &x509.Certificate{
		SerialNumber: sn,
		Subject:      entity,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Append subjectAltNames
	serverCert.DNSNames = c.DNSNames
	if len(c.IP) == 0 {
		for _, name := range c.DNSNames {
			ips, err := net.LookupIP(name)
			if err != nil {
				return trace.Wrap(err)
			}

			if ips != nil {
				serverCert.IPAddresses = append(serverCert.IPAddresses, ips...)
			}
		}
	} else {
		for _, ip := range c.IP {
			serverCert.IPAddresses = append(serverCert.IPAddresses, net.ParseIP(ip))
		}
	}

	// Generate CA key and certificate
	caPK, err := rsa.GenerateKey(rand.Reader, c.Length)
	if err != nil {
		return trace.Wrap(err)
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caPK.PublicKey, caPK)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.writeKeyAndCert(path.Join(c.Out, "ca"), caCertBytes, caPK)
	if err != nil {
		return trace.Wrap(err)
	}

	// Generate server key and certificate
	serverPK, err := rsa.GenerateKey(rand.Reader, c.Length)
	if err != nil {
		return trace.Wrap(err)
	}

	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverCert, caCert, &serverPK.PublicKey, caPK)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.writeKeyAndCert(path.Join(c.Out, "server"), serverCertBytes, serverPK)
	if err != nil {
		return trace.Wrap(err)
	}

	// Generate client key and certificate
	clientPK, err := rsa.GenerateKey(rand.Reader, c.Length)
	if err != nil {
		return trace.Wrap(err)
	}

	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientCert, caCert, &clientPK.PublicKey, caPK)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.writeKeyAndCert(path.Join(c.Out, "client"), clientCertBytes, clientPK)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// writeKeyAndCert writes private key and certificate on disk
func (c *GenCertsCmd) writeKeyAndCert(prefix string, certBytes []byte, pk *rsa.PrivateKey) error {
	caBytesPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	caPkBytesPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})

	err := ioutil.WriteFile(prefix+".crt", caBytesPEM, 0444)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(prefix+".key", caPkBytesPEM, 0444)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
