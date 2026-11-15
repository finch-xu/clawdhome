package vm

import (
	"os"
	"path/filepath"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

// RouteLabel 返回出口路由的界面文案。
func RouteLabel(route egress.Route) string {
	switch route {
	case egress.RouteClash:
		return "Clash"
	case egress.RouteSystem:
		return "系统路由"
	case egress.RouteBlocked:
		return "断开公网"
	default:
		return "未知"
	}
}

// SetRoute 原子写入首选出口路由，网关下次读取时生效。
func (l Launcher) SetRoute(route egress.Route) error {
	file, err := os.CreateTemp(l.Home, ".route-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.WriteString(string(route) + "\n")
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), filepath.Join(l.Home, egress.RouteFile))
}
