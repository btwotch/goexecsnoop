package goexecsnoop

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type TraceProcStap struct {
	tpm     *TraceProcMonitor
	stdout  io.Reader
	stderr  io.Reader
	cmd     *exec.Cmd
	started sync.Mutex
	running atomic.Value
}

func (tps *TraceProcStap) setTraceProcMonitor(tpm *TraceProcMonitor) {
	tps.tpm = tpm
}

func (tps *TraceProcStap) processEventLine(line string) {
	strs := strings.Split(line, "/")

	if line == "s/" {
		tps.started.Unlock()
	} else if strs[0] == "f" {
		ppid64, err := strconv.ParseUint(strs[1], 10, 32)
		if err != nil {
			panic(err)
		}
		pid64, err := strconv.ParseUint(strs[2], 10, 32)
		if err != nil {
			panic(err)
		}

		pid := uint32(pid64)
		ppid := uint32(ppid64)

		_, parentOk := tps.tpm.childPids[ppid]
		_, childOk := tps.tpm.childPids[pid]
		if !parentOk && !childOk && !tps.tpm.allPids {
			return
		}

		tps.tpm.childPids[pid] = struct{}{}

		lastPidIteration := tps.tpm.lastPidIteration(pid) + 1

		tpi := traceProcIndex{Pid: pid, Iteration: uint(lastPidIteration)}

		tp := TraceProc{Pid: pid, Ppid: ppid, StartTime: time.Now().UnixNano()}

		tps.tpm.processMapLock.Lock()
		tps.tpm.processMap[tpi] = &tp
		tps.tpm.processMapLock.Unlock()

	} else if strs[0] == "e" {
		var params []string
		var cmd string

		ppid64, err := strconv.ParseUint(strs[3], 10, 32)
		if err != nil {
			panic(err)
		}
		pid64, err := strconv.ParseUint(strs[4], 10, 32)
		if err != nil {
			panic(err)
		}

		pid := uint32(pid64)
		ppid := uint32(ppid64)

		args := strings.Split(strings.Join(strs[6:], "/"), " ")
		if len(args) == 0 {
			cmd = strs[5]
		} else {
			cmdPath := strings.Split(args[0], "/")
			cmd = cmdPath[len(cmdPath)-1]
		}

		if len(args) > 1 {
			params = args[1:]
		}

		tp := tps.tpm.latestTraceProc(pid)
		if tp == nil {
			_, ok := tps.tpm.childPids[ppid]
			if !ok && !tps.tpm.allPids {
				return
			}
			tps.tpm.childPids[pid] = struct{}{}
			lastPidIteration := tps.tpm.lastPidIteration(pid) + 1

			tpi := traceProcIndex{Pid: pid, Iteration: uint(lastPidIteration)}

			tp = &TraceProc{Pid: pid, Ppid: ppid, StartTime: time.Now().UnixNano()}

			tps.tpm.processMapLock.Lock()
			tps.tpm.processMap[tpi] = tp
			tps.tpm.processMapLock.Unlock()

		}

		tps.tpm.processMapLock.Lock()
		tp.Ppid = ppid
		tp.Pid = pid
		tp.Cmd = cmd
		tp.Params = params
		tps.tpm.processMapLock.Unlock()

	} else if strs[0] == "x" {
		pid64, err := strconv.ParseUint(strs[1], 10, 32)
		if err != nil {
			panic(err)
		}
		code64, err := strconv.ParseUint(strs[2], 10, 32)
		if err != nil {
			panic(err)
		}

		pid := uint32(pid64)
		code := uint32(code64)

		tp := tps.tpm.latestTraceProc(pid)
		if tp == nil {
			return
		}

		tps.tpm.processMapLock.Lock()
		tp.ExitCode = code
		tp.EndTime = time.Now().UnixNano()
		tps.tpm.processMapLock.Unlock()

		delete(tps.tpm.childPids, pid)
	}
}

func (tps *TraceProcStap) processEvents() {
	scanner := bufio.NewScanner(tps.stdout)
	for scanner.Scan() {
		str := scanner.Text()

		running := tps.running.Load().(bool)
		if !running {
			return
		}
		tps.processEventLine(str)
	}
}

var (
	execsnoopStapCmd = `
probe begin
{
	printf("s/\n")
}

probe kprobe.function(@arch_syscall_prefix "sys_clone").return
{
	printf("f/%d/%d\n", ppid(), pid())
}

probe nd_syscall.exit_group
{
	sig = status & 0x7F
        code = sig ? sig : status >> 8
	printf("x/%d/%ld\n", pid(), code)
}

probe nd_syscall.execve.return
{
	printf("e/%ld/%d/%d/%d/%s/%s\n", gettimeofday_ns(), uid(),
	    ppid(), pid(), execname(), cmdline_str());
}

`
)

func (tps *TraceProcStap) Start() {
	tps.started.Lock()

	var cmd *exec.Cmd

	cmd = exec.Command("stap", "-")
	cmd.Stdin = bytes.NewBuffer([]byte(execsnoopStapCmd))

	tps.stdout, cmd.Stdout = io.Pipe()
	tps.stderr, cmd.Stderr = io.Pipe()

	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	tps.running.Store(true)

	tps.cmd = cmd

	go func() {
		err := cmd.Wait()
		if err != nil {
			panic(err)
		}
	}()
	go tps.processEvents()

	tps.started.Lock()
}

func (tps *TraceProcStap) Stop() {
	tps.running.Store(false)
	err := tps.cmd.Process.Kill()
	if err != nil {
		panic(err)
	}
}
