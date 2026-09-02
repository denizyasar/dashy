package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Provider struct {
	salt string
	data map[string]string
}

func New(path string, salt string) (*Provider, error) {
	provider := &Provider{
		salt: salt,
		data: make(map[string]string),
	}

	usersBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(usersBytes, &provider.data)
	if err != nil {
		return nil, err
	}

	rand.Seed(time.Now().UTC().UnixNano())
	for username := range provider.data {

		if !strings.HasPrefix(provider.data[username], "###") {

			pepper := make([]byte, 16)
			_, err = rand.Read(pepper)
			if err != nil {
				return nil, err
			}

			hasher := sha256.New()
			hasher.Write([]byte(salt))
			hasher.Write([]byte(provider.data[username]))
			hasher.Write(pepper)

			provider.data[username] = "###" + hex.EncodeToString(pepper) + hex.EncodeToString(hasher.Sum(nil))
		}
	}

	usersBytes, err = json.MarshalIndent(provider.data, "", "  ")
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(path, usersBytes, 0600)
	if err != nil {
		return nil, err
	}

	for username := range provider.data {
		provider.data[username] = provider.data[username][3:]
	}
	return provider, nil
}

func (p *Provider) IsValid(username string, password string) bool {
	if value, exist := p.data[username]; exist {

		if len(value) != 96 {
			return false
		}

		pepper, err := hex.DecodeString(value[:32])
		if err != nil {

			return false
		}

		hasher := sha256.New()
		hasher.Write([]byte(p.salt))
		hasher.Write([]byte(password))
		hasher.Write(pepper)

		hash, err := hex.DecodeString(value[32:])
		if err != nil {
			return false
		}

		if subtle.ConstantTimeCompare(hash, hasher.Sum(nil)) != 1 {

			return false
		}

		return true
	}
	return false
}

func (p *Provider) Handler(c *gin.Context) {

	username, password, ok := c.Request.BasicAuth()
	if !ok || !p.IsValid(username, password) {
		c.Header("WWW-Authenticate", `Basic realm="Please Login:"`)
		c.AbortWithError(401, errors.New("unauthorized"))
		return
	}

}
