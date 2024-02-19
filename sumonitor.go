package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
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
				go traceSUProcess(pid)
				processedPIDs[pid] = struct{}{}
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
	cmdline, _ := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	return regexp.MustCompile(`su`).MatchString(strings.ReplaceAll(string(cmdline), "\x00", " "))
}

func traceSUProcess(pid int) {
	// Add your logic to trace and monitor the su process as needed
	fmt.Printf("Tracing su process with PID: %d\n", pid)
	// ...
}

func main() {
	go watchSUProcesses()

	time.Sleep(10 * time.Minute) // Adjust the duration based on your needs

	close(shutdownChan)
	time.Sleep(2 * time.Second) // Allow some time for goroutines to finish before exiting
	fmt.Println("Exiting the main program.")
}
