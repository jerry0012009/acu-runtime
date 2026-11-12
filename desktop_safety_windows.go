package main

import (
	"errors"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func codexDesktopOpen() (bool, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "ChatGPT.exe") {
			continue
		}
		process, openErr := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if openErr != nil {
			return false, openErr
		}
		buf := make([]uint16, 32768)
		size := uint32(len(buf))
		queryErr := windows.QueryFullProcessImageName(process, 0, &buf[0], &size)
		windows.CloseHandle(process)
		if queryErr != nil {
			return false, queryErr
		}
		path := strings.ToLower(windows.UTF16ToString(buf[:size]))
		if strings.Contains(path, `\windowsapps\openai.codex_`) && strings.HasSuffix(path, `\app\chatgpt.exe`) {
			return true, nil
		}
	}
	if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return false, nil
	}
	return false, err
}
