package sources_test

import (
	"fmt"
	"testing"

	"github.com/NorthlandPowerEurope/dashy/agent/sources"
)

func TestNewOmcAdapter(t *testing.T) {

	a, _ := sources.Open("omc", map[string]string{
		"address":  "http://10.113.31.24:5000/recentvalues.xml?locID=113",
		"username": "SUPERVISOR",
		"password": "4321",
		"timezone": "UTC",
	})

	fmt.Println(a.Values())

}
