package goexecsnoop

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
	"testing"
	"time"
)

func TestFillerStap(t *testing.T) {
	tpm := NewTraceProcMonitorStap(100)

	tps := tpm.tpf.(*TraceProcStap)

	stapOutput := `
s/
f/100/1001
e/1337/0/100/1001/bash/bin/bash
e/1337/0/100/1001/ls/bin/ls --color=auto /*
f/100/1002
e/1337/0/100/1002/false/bin/false
x/1002/1
# wrap around of pids
f/100/1002
e/1337/0/100/1002/true/bin/true
x/1002/0
f/100/1002
f/100/1002
f/100/1002
`

	lines := strings.Split(stapOutput, "\n")

	tps.started.Lock()
	for _, line := range lines {
		tps.processEventLine(line)
	}

	procs := tpm.Processes()

	for i, cmd := range []string{"ls", "false", "true"} {
		if procs[i].Cmd != cmd {
			t.Errorf("Expected cmd of %+v to be %s, but is %s", procs[i], cmd, procs[i].Cmd)
		}
	}

	for i, exitCode := range []uint32{0, 1, 0} {
		if procs[i].ExitCode != exitCode {
			t.Errorf("Expected exitcode of %+v to be %d, but is %d", procs[i], exitCode, procs[i].ExitCode)
		}
	}

	for i, params := range [][]string{[]string{"--color=auto", "/*"}} {
		for j, param := range params {
			if procs[i].Params[j] != param {
				t.Errorf("Expected param %d of %+v to be %s, but is %s", j, procs[i], param, procs[i].Params[j])
			}
		}
	}
}

func isRoot(t *testing.T) bool {
	currentUser, err := user.Current()
	if err != nil {
		t.Errorf("[isRoot] Unable to get current user: %s", err)
	}
	return currentUser.Uid == "0"
}

func TestTraceProcMonitorStap(t *testing.T) {
	if !isRoot(t) {
		t.Skip("Test can only be run as root")
	}

	pid := uint32(os.Getpid())
	tpm := NewTraceProcMonitorStap(pid)
	tpm.Start()

	var procs []*TraceProc

	procIndex := -1
	for i := 0; i < 60 && procIndex == -1; i++ {
		cmd := exec.Command("/bin/true", "foo/bar", "baz")
		cmd.Run()

		procs = tpm.Processes()
		for i, proc := range procs {
			if proc.Cmd == "true" {
				procIndex = i
			}
		}
		time.Sleep(1 * time.Second)
	}

	tpm.Stop()

	if procs[procIndex].Cmd != "true" {
		t.Errorf("Expected cmd of %+v to be %s, but is %s", procs[procIndex], "true", procs[procIndex].Cmd)
	}

	if procs[procIndex].Params[0] != "foo/bar" {
		t.Errorf("Expected param 0 of %+v to be %s, but is %s", procs[procIndex], "foo/bar", procs[procIndex].Params[0])
	}
	if procs[procIndex].Params[1] != "baz" {
		t.Errorf("Expected param 1 of %+v to be %s, but is %s", procs[procIndex], "baz", procs[procIndex].Params[1])
	}
}
