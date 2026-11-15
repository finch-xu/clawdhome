package vm

import (
	"fmt"
	"os"
	"path/filepath"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

// routeLabel 给出口路由一个名字，用在本包生成的诊断文案里。界面自己的路由
// 名称走 UI 层的文案字典，不用这个函数。
func routeLabel(route egress.Route) string {
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
		return fmt.Errorf("无法创建出口路由临时文件: %w", err)
	}
	defer os.Remove(file.Name())
	_, err = file.WriteString(string(route) + "\n")
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("无法写入出口路由: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("无法关闭出口路由文件: %w", closeErr)
	}
	if err := os.Rename(file.Name(), filepath.Join(l.Home, egress.RouteFile)); err != nil {
		return fmt.Errorf("无法保存出口路由设置: %w", err)
	}
	return nil
}
