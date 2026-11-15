//go:build windows

package platform

import "errors"

func CodesignVerify(string) error { return errors.New("bu işletim sisteminde desteklenmiyor") }
func RelaunchApp(string) error    { return errors.New("bu işletim sisteminde desteklenmiyor") }
