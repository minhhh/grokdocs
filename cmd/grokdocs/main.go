package main

import (
	"os"
	"runtime/pprof"
)

func main() {
	if cpuFile := os.Getenv("PPROF_CPU"); cpuFile != "" {
		f, _ := os.Create(cpuFile)
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if memFile := os.Getenv("PPROF_MEM"); memFile != "" {
		defer func() {
			f, _ := os.Create(memFile)
			pprof.WriteHeapProfile(f)
			f.Close()
		}()
	}
	Execute()
}
