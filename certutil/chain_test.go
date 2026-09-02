package certutil

import (
	"crypto/x509"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestBundle(t *testing.T) {
	_, err := Bundle("./testdata/starfield.crt", false)
	if err != nil {
		t.Error(err)
	}
}

func TestFetchCertificateChain(t *testing.T) {
	file, err := os.Open("./testdata/comodo.der.crt")
	if err != nil {
		t.Error(err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		t.Error(err)
	}

	cert, err := x509.ParseCertificate(data)
	if err != nil {
		t.Error(err)
	}

	certs, err := FetchCertificateChain(cert)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, 3, len(certs))
}
