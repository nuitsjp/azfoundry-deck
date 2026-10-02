//go:build windows && !server

package main

import (
	"syscall"
	"unsafe"
)

func showStartupFailure() {
	title, _ := syscall.UTF16PtrFromString("AzFoundryDeckを起動できません")
	message, _ := syscall.UTF16PtrFromString("初期化に失敗しました。設定とデータ領域のアクセス権を確認してください。")
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
