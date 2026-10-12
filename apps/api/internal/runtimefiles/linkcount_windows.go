//go:build windows

package runtimefiles

import (
	"os"

	"golang.org/x/sys/windows"
)

func hasSingleLink(path string, _ os.FileInfo) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	handle, err := windows.CreateFile(
		name,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	var information windows.ByHandleFileInformation
	return windows.GetFileInformationByHandle(handle, &information) == nil && information.NumberOfLinks == 1
}
