package core

import "errors"

// TUN's DNS hijack must have an enabled resolver, matching the TUN/DNS
// switch linkage. Preserve supplied upstreams and policies; fill only absent or
// empty upstream lists using controlled-config defaults.
func ensureTunDNS(cfg map[string]any) error {
	dns, ok := cfg["dns"].(map[string]any)
	if !ok && cfg["dns"] != nil {
		return errors.New("配置中的 dns 必须为对象")
	}
	if dns == nil {
		dns = map[string]any{}
	}
	dns["enable"] = true
	fillDNSDefaults(dns)
	cfg["dns"] = dns
	return nil
}

func fillDNSDefaults(dns map[string]any) {
	if emptyDNSList(dns["nameserver"]) {
		dns["nameserver"] = []string{"https://doh.pub/dns-query", "https://dns.alidns.com/dns-query"}
	}
	if emptyDNSList(dns["default-nameserver"]) {
		dns["default-nameserver"] = []string{"tls://223.5.5.5"}
	}
}

// ensureTunSniffer enables TLS/HTTP/QUIC domain sniffing for TUN when the user
// has not explicitly turned sniffer off, so connections whose DNS bypassed port
// 53 (DoH, or router/enterprise DNS on a local subnet) still recover their
// hostname from SNI/Host for rules and the connections list.
func ensureTunSniffer(cfg map[string]any, explicit *bool) error {
	if explicit != nil && !*explicit {
		return nil
	}
	section, ok := cfg["sniffer"].(map[string]any)
	if !ok && cfg["sniffer"] != nil {
		return errors.New("配置中的 sniffer 必须为对象")
	}
	if section == nil {
		section = map[string]any{}
	}
	section["enable"] = true
	fillSnifferDefaults(section)
	cfg["sniffer"] = section
	return nil
}

func fillSnifferDefaults(section map[string]any) {
	if section["sniff"] == nil {
		section["sniff"] = map[string]any{
			"HTTP": map[string]any{"ports": []int{80, 8080, 8880}, "override-destination": true},
			"TLS":  map[string]any{"ports": []int{443, 8443}},
			"QUIC": map[string]any{"ports": []int{443, 8443}},
		}
	}
}

func emptyDNSList(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case []any:
		return len(v) == 0
	case []string:
		return len(v) == 0
	}
	return false
}

func enabledTunDNS(options CoreOptions) CoreOptions {
	dns := DNSOptions{}
	if options.DNS != nil {
		dns = *options.DNS
	}
	enabled := true
	dns.Enabled = &enabled
	options.DNS = &dns
	return options
}
