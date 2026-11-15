package vm

import (
	"fmt"
	"os"
	"path/filepath"

	egress "github.com/finch-xu/clawdhome/lima-egress"
)

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
