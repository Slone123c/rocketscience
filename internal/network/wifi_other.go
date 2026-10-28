//go:build !darwin

package network

import "errors"

func Read() (Snapshot, error) {
	return Snapshot{}, errors.New("当前平台不支持读取 Wi-Fi 状态")
}
func RequestAccess() error { return errors.New("当前平台不支持 Wi-Fi 授权") }
