package sources

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type RandSource struct {
	funcs map[string]func() string
}

func NewRandSource(params map[string]string) (*RandSource, error) {
	funcs := make(map[string]func() string)
	for k, v := range params {
		parts := strings.Split(v, " ")
		dt := parts[0]
		unit := ""
		if len(parts) > 1 {
			unit = parts[1]
		}
		switch dt {
		case "uint16":
			funcs[k] = func() string {
				return strconv.FormatUint(uint64(rand.Intn(65536)), 10) + " " + unit
			}
		case "float64":
			funcs[k] = func() string {
				return strconv.FormatFloat(rand.Float64(), 'f', -1, 64) + " " + unit
			}

		case "string":
			funcs[k] = func() string {
				return fmt.Sprintf("random_string_%d %s", rand.Intn(100), unit)
			}
		default:
			return nil, fmt.Errorf("unsupported data type: %s", dt)
		}
	}

	return &RandSource{
		funcs: funcs,
	}, nil
}

func (a *RandSource) Values() (map[string]Value, error) {
	values := make(map[string]Value)
	for k, f := range a.funcs {
		value := f()
		values[k] = Value{
			Timestamp: time.Now(),
			Value:     value,
		}
	}
	for k, v := range values {
		log.Info().Str("key", k).Str("value", v.Value).Msg("Generated random value")
	}
	return values, nil
}

func (a *RandSource) Close() error {
	// No resources to close for RandSource
	return nil
}
