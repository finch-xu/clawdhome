package egress

import (
	"os"
	"path/filepath"
	"strings"
)

type Route string

const (
	RouteClash   Route = "clash"
	RouteSystem  Route = "system"
	RouteBlocked Route = "blocked"
	RouteFile          = "mvp-route"
)

// ReadRoute defaults to the host's system route. Invalid or unreadable
// configuration fails closed instead of changing the outbound path.
func ReadRoute(dir string) Route {
	if dir == "" {
		return RouteSystem
	}
	data, err := os.ReadFile(filepath.Join(dir, RouteFile))
	if os.IsNotExist(err) {
		return RouteSystem
	}
	if err == nil {
		switch route := Route(strings.TrimSpace(string(data))); route {
		case RouteClash, RouteSystem, RouteBlocked:
			return route
		}
	}
	return RouteBlocked
}
