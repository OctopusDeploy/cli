//go:build windows

package shell

import (
	"os"
	"syscall"
	"unsafe"
)

// parentProcessName returns the file name of the executable that launched us.
func parentProcessName() string {
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(snapshot)

	entry := syscall.ProcessEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	ppid := uint32(os.Getppid())

	for err = syscall.Process32First(snapshot, &entry); err == nil; err = syscall.Process32Next(snapshot, &entry) {
		if entry.ProcessID == ppid {
			return syscall.UTF16ToString(entry.ExeFile[:])
		}
	}
	return ""
}
