package egress

import "time"

const (
	ProxyWaiting   = "waiting"
	ProxyConnected = "connected"
	ProxyFailed    = "failed"
	ProxyDisabled  = "disabled"

	ProbeWaiting     = "waiting"
	ProbeReachable   = "reachable"
	ProbeUnreachable = "unreachable"
	ProbeExpired     = "expired"
	ProbeSkipped     = "skipped"
)

// GatewayStatus is the snapshot shared with the launcher over the instance directory.
type GatewayStatus struct {
	PID        int       `json:"pid"`
	Running    bool      `json:"running"`
	Route      Route     `json:"route"`
	ProxyAddr  string    `json:"proxyAddr"`
	Proxy      string    `json:"proxy"`
	ProxyError string    `json:"proxyError,omitempty"`
	Probe      string    `json:"probe"`
	Reason     string    `json:"reason"`
	LastProbe  time.Time `json:"lastProbe"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Online     bool      `json:"online"`
	Until      time.Time `json:"until"`
}
