package sources_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/NorthlandPowerEurope/dashy/agent/sources"
)

func TestRADACAdapterExtract(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/RADAC_TEST_20210908.txt")
	if err != nil {
		t.Fatal(err)
		t.FailNow()
	}

	a, err := sources.NewRADACAdapter(map[string]string{
		"address":  "http://10.112.19.2",
		"paths":    "data/height/Tmax/;data/height/Tm02/",
		"factors":  "0.1;0.1",
		"timezone": "UTC",
	})
	if err != nil {
		t.Fatal(err)
		t.FailNow()
	}

	ts, val, err := a.ExtractLastEntry(bytes.NewBuffer(b))
	if err != nil {
		t.Fatal(err)
		t.FailNow()
	}

	fmt.Println(ts, val)
}
