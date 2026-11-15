package vm

import (
	"os"
	"path/filepath"
	"testing"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

func TestSetRouteWritesSelectedRoute(t *testing.T) {
	for _, route := range []egress.Route{egress.RouteClash, egress.RouteSystem, egress.RouteBlocked} {
		home := t.TempDir()
		l := Launcher{Home: home}
		if err := l.SetRoute(route); err != nil {
			t.Fatalf("SetRoute(%q): %v", route, err)
		}
		raw, err := os.ReadFile(filepath.Join(home, egress.RouteFile))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := string(raw), string(route)+"\n"; got != want {
			t.Fatalf("route file = %q, want %q", got, want)
		}
	}
}

func TestRouteLabelCoversEveryRoute(t *testing.T) {
	labels := map[egress.Route]string{
		egress.RouteClash:   "Clash",
		egress.RouteSystem:  "系统路由",
		egress.RouteBlocked: "断开公网",
		egress.Route("???"): "未知",
	}
	for route, want := range labels {
		if got := routeLabel(route); got != want {
			t.Fatalf("RouteLabel(%q) = %q, want %q", route, got, want)
		}
	}
}
