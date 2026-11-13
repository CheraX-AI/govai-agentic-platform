// Package devsetup is a local-only provisioning tool. The API does not import it.
package devsetup

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrSetup = errors.New("local authentication setup failed; see the fixed stage identifier")

func serial() *big.Int {
	n, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		panic(ErrSetup)
	}

	return n
}

func newKey() *ecdsa.PrivateKey {
	k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		panic(ErrSetup)
	}

	return k
}

func writeNew(path string, b []byte) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return ErrSetup
	}

	_, e = f.Write(b)
	ce := f.Close()
	if e != nil || ce != nil {
		return ErrSetup
	}

	return nil
}

// validHostname accepts a plain DNS name such as govai.example.ac.lk:
// letters, digits and hyphens in dot-separated labels, no wildcard or port.
func validHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}

	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}

	return true
}

// Certificates refuses an existing directory and never installs system/browser
// trust. CA signing key exists in memory only; server private keys remain local.
// The certificates always cover localhost; extraHosts adds the VM's DNS names
// or IP addresses (for example govai.example.ac.lk or 10.0.0.5).
func Certificates(dir string, extraHosts ...string) (string, error) {
	dnsNames := []string{"localhost"}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	for _, host := range extraHosts {
		if ip := net.ParseIP(host); ip != nil {
			ips = append(ips, ip)
		} else if validHostname(host) {
			dnsNames = append(dnsNames, strings.ToLower(host))
		} else {
			return "", ErrSetup
		}
	}

	if os.Mkdir(dir, 0700) != nil {
		return "", ErrSetup
	}

	now := time.Now()
	rootKey := newKey()
	root := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "GovAI Local Development CA"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(1, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}
	rootDER, e := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)
	if e != nil {
		return "", ErrSetup
	}

	if writeNew(
		filepath.Join(dir, "ca.pem"),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}),
	) != nil {
		return "", ErrSetup
	}

	for _, name := range []string{"keycloak", "api"} {
		key := newKey()
		leaf := &x509.Certificate{
			SerialNumber: serial(),
			Subject:      pkix.Name{CommonName: "GovAI Local " + name},
			NotBefore:    now.Add(-time.Hour),
			NotAfter:     now.Add(90 * 24 * time.Hour),
			DNSNames:     dnsNames,
			IPAddresses:  ips,
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		der, e := x509.CreateCertificate(rand.Reader, leaf, root, &key.PublicKey, rootKey)
		if e != nil {
			return "", ErrSetup
		}

		private, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil {
			return "", ErrSetup
		}

		if writeNew(
			filepath.Join(dir, name+"-cert.pem"),
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		) != nil ||
			writeNew(
				filepath.Join(dir, name+"-key.pem"),
				pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}),
			) != nil {
			return "", ErrSetup
		}
	}

	sum := sha256.Sum256(rootDER)
	return hex.EncodeToString(sum[:]), nil
}
