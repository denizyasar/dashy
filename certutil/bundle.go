package certutil

import (
	"os"
	"path/filepath"
	"strings"
)

func Bundle(inputFile string, includeSystem bool) (string, error) {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return "", err
	}

	cert, err := DecodeCertificate(data)
	if err != nil {
		return "", err
	}

	certs, err := FetchCertificateChain(cert)
	if err != nil {
		return "", err
	}

	if includeSystem {
		certs, err = AddRootCA(certs)
		if err != nil {
			return "", err
		}
	}

	inputExt := filepath.Ext(inputFile)
	inputInfo, err := os.Stat(inputFile)
	if err != nil {
		return "", err
	}
	outputFile := strings.TrimSuffix(inputFile, inputExt) + ".bundle" + inputExt
	err = os.WriteFile(outputFile, EncodeCertificates(certs), inputInfo.Mode())
	if err != nil {
		return "", err
	}

	return outputFile, nil
}
