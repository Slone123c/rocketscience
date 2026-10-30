package tunhelper

import "testing"

func TestValidateRejectsWhatRootMustNotBuild(t *testing.T) {
	good := Request{MTU: 9000, Inet4Address: []string{"198.18.0.1/30"}, AutoRoute: true}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Request){
		"name":    func(r *Request) { r.Name = "en0" },
		"mtu":     func(r *Request) { r.MTU = 100 },
		"address": func(r *Request) { r.Inet4Address = nil },
		"route":   func(r *Request) { r.RouteAddress = []string{"not-a-prefix"} },
		"dns":     func(r *Request) { r.DNSMode = "anything" },
	} {
		request := good
		change(&request)
		if request.Validate() == nil {
			t.Errorf("%s: accepted %+v", name, request)
		}
	}
}
