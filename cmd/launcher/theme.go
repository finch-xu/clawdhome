package main

import "github.com/egoist/mygo"

// toThemeSource 把保存的外观偏好映射到 MyGo 的 ThemeSource。空值和未知值都
// 回到跟随系统。
func toThemeSource(pref string) mygo.ThemeSource {
	switch pref {
	case string(mygo.ThemeLight):
		return mygo.ThemeLight
	case string(mygo.ThemeDark):
		return mygo.ThemeDark
	default:
		return mygo.ThemeSystem
	}
}
