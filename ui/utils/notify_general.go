//go:build (linux && !android) || ios || darwin || openbsd || freebsd || netbsd
// +build linux,!android ios darwin openbsd freebsd netbsd

package utils

import "github.com/crypto-power/cryptopower/ui/notify"

// Create notifier
func CreateNewNotifierWithIcon(iconPath string) (notifier notify.Notifier, err error) {
	return CreateNewNotifier()
}
