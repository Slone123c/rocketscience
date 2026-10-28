package network

// Known distinguishes a confirmed non-Wi-Fi network from a name hidden by privacy permissions.
type Snapshot struct {
	Known  bool   `json:"known"`
	SSID   string `json:"ssid"`
	Detail string `json:"detail"`
}
