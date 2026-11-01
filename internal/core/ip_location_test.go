package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPLocationParsesProvidersAndFallsBack(t *testing.T) {
	info, err := parseIPInfo([]byte(`{"ip":"1.2.3.4","city":"Guangzhou","region":"Guangdong","country":"CN","loc":"23.1167,113.2500"}`))
	if err != nil || info.City != "Guangzhou" || info.Lat != 23.1167 || info.Lon != 113.25 {
		t.Fatalf("ipinfo = %+v, %v", info, err)
	}
	if _, err := parseIPInfo([]byte(`{"bogon":true}`)); err == nil {
		t.Fatal("accepted a response without coordinates")
	}
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }))
	defer limited.Close()
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"city":"Tokyo","country_code":"JP","latitude":35.69,"longitude":139.69}`))
	}))
	defer fallback.Close()
	saved := ipLocationProviders
	defer func() { ipLocationProviders = saved }()
	ipLocationProviders = []ipLocationProvider{{"first", func(string) string { return limited.URL }, parseIPInfo}, {"second", func(ip string) string { return fallback.URL + "/" + ip }, parseIPAPI}}
	value := lookupIPLocation(http.DefaultClient, "")
	if got := ipLocationProviders[0].url("1.2.3.4"); got != limited.URL {
		t.Fatalf("url = %q", got)
	}
	if !value.Known || value.Source != "second" || value.City != "Tokyo" || value.Lon != 139.69 {
		t.Fatalf("fallback = %+v", value)
	}
	ipLocationProviders = ipLocationProviders[:1]
	if value := lookupIPLocation(http.DefaultClient, ""); value.Known || value.Error == "" {
		t.Fatalf("failure = %+v", value)
	}
}

func TestIPAPIComParsesCityAndRejectsFailure(t *testing.T) {
	got, err := parseIPAPICom([]byte(`{"status":"success","countryCode":"US","regionName":"California","city":"San Francisco","lat":37.77,"lon":-122.42}`))
	if err != nil || got.City != "San Francisco" || got.Country != "US" || got.Lon != -122.42 {
		t.Fatalf("ip-api.com = %+v, %v", got, err)
	}
	if _, err := parseIPAPICom([]byte(`{"status":"fail","message":"reserved range"}`)); err == nil {
		t.Fatal("accepted a failed lookup")
	}
	if ipAPIURL("") != "" || ipAPIURL("1.2.3.4") == "" {
		t.Fatal("ip-api.com must require a concrete IP")
	}
}
