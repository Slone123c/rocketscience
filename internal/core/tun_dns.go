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
