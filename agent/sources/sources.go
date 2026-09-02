package sources

import (
	"errors"
	"time"
)

type Value struct {
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value"`
}

type Source interface {
	Values() (map[string]Value, error)
	Close() error
}

func Open(t string, config map[string]string) (Source, error) {
	switch t {
	case "omc":
		return NewOmcAdapter(config)
	case "radac":
		return NewRADACAdapter(config)
	case "rand":
		return NewRandSource(config)
	}
	return nil, errors.New("cannot find source of type '" + t + "'")
}

func checkParams(params map[string]string, mustExist ...string) error {
	for _, param := range mustExist {
		if _, exist := params[param]; !exist {
			return errors.New("parameter '" + param + "' does not exist")
		}
	}
	return nil
}
