package main

import (
	"fmt"
	"time"

	goexecsnoop "github.com/btwotch/goexecsnoop"
)

func main() {
	tpm := goexecsnoop.NewTraceProcMonitorStap(uint32(0))

	tpm.Start()

	fmt.Println("monitoring for 10 seconds ...")
	time.Sleep(10 * time.Second)

	tpm.Stop()

	procs := tpm.Processes()

	for _, p := range procs {
		fmt.Printf("- %+v\n", p)
	}
}
