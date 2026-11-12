package main

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"sync"
	"unsafe"
)

// Start only a headless inspection/configuration command. Its descendants
// belong to this job before its first instruction runs; closing the job cannot
// affect the already-running desktop or another agent.
func startOwnedCommand(c *exec.Cmd) (func(), error) {
	hideProcess(c)
	c.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	if err = c.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	fail := func(e error) (func(), error) { c.Process.Kill(); c.Wait(); windows.CloseHandle(job); return nil, e }
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(c.Process.Pid))
	if err != nil {
		return fail(err)
	}
	err = windows.AssignProcessToJobObject(job, process)
	windows.CloseHandle(process)
	if err != nil {
		return fail(err)
	}
	threads, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fail(err)
	}
	defer windows.CloseHandle(threads)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	err = windows.Thread32First(threads, &entry)
	for err == nil {
		if entry.OwnerProcessID == uint32(c.Process.Pid) {
			thread, e := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if e != nil {
				return fail(e)
			}
			_, e = windows.ResumeThread(thread)
			windows.CloseHandle(thread)
			if e != nil {
				return fail(e)
			}
			var once sync.Once
			return func() { once.Do(func() { windows.CloseHandle(job) }) }, nil
		}
		err = windows.Thread32Next(threads, &entry)
	}
	return fail(err)
}
