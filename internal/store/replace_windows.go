//go:build windows

package store

import (
	"syscall"
	"unsafe"
)

const (
	movefileReplaceExisting = 0x1
	movefileWriteThrough    = 0x8
)

func replaceFile(from, to string) error {
	src, err := syscall.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	dst, err := syscall.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	r1, _, callErr := procMoveFileExW.Call(
		uintptr(unsafe.Pointer(src)),
		uintptr(unsafe.Pointer(dst)),
		uintptr(movefileReplaceExisting|movefileWriteThrough),
	)
	if r1 == 0 {
		if callErr != syscall.Errno(0) {
			return callErr
		}
		return syscall.EINVAL
	}
	return nil
}

var procMoveFileExW = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
