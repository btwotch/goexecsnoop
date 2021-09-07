package goexecsnoop

import (
	"math"
	"sort"
	"sync"
)

type TraceProcFiller interface {
	Start()
	setTraceProcMonitor(tpm *TraceProcMonitor)
	Stop()
}

type traceProcIndex struct {
	Pid       uint32
	Iteration uint
}

type TraceProc struct {
	// only the last execv is saved
	Cmd       string
	Params    []string
	Pid       uint32
	Ppid      uint32
	ExitCode  uint32
	StartTime int64
	EndTime   int64
}

type ByStartTime []*TraceProc

func (bst ByStartTime) Len() int {
	return len(bst)
}

func (bst ByStartTime) Swap(i, j int) {
	bst[i], bst[j] = bst[j], bst[i]
}

func (bst ByStartTime) Less(i, j int) bool {
	return bst[i].StartTime < bst[j].StartTime
}

type TraceProcMonitor struct {
	processMap     map[traceProcIndex]*TraceProc
	processMapLock sync.Mutex
	childPids      map[uint32]struct{}
	allPids        bool
	tpf            TraceProcFiller
}

func (t *TraceProcMonitor) Processes() []*TraceProc {
	t.processMapLock.Lock()
	defer t.processMapLock.Unlock()

	p := make([]*TraceProc, 0)

	for _, tp := range t.processMap {
		if tp.Cmd == "" {
			continue
		}
		p = append(p, tp)

	}

	sort.Sort(ByStartTime(p))
	return p
}

func (t *TraceProcMonitor) lastPidIteration(pid uint32) int {
	t.processMapLock.Lock()
	defer t.processMapLock.Unlock()

	tpi := traceProcIndex{Pid: pid, Iteration: 0}

	for i := uint(0); i < math.MaxInt32; i++ {
		tpi.Iteration = i
		if _, ok := t.processMap[tpi]; !ok {
			return int(i) - 1
		}
	}

	panic("PidIterationOverflow")
}

func (t *TraceProcMonitor) latestTraceProc(pid uint32) *TraceProc {
	tpi := traceProcIndex{Pid: pid, Iteration: uint(t.lastPidIteration(pid))}
	t.processMapLock.Lock()
	ret := t.processMap[tpi]
	t.processMapLock.Unlock()
	return ret
}

func NewTraceProcMonitorStap(pid uint32) *TraceProcMonitor {
	var tpm TraceProcMonitor

	tpm.allPids = pid == 0

	tpm.init()

	tpm.childPids[pid] = struct{}{}

	var tps TraceProcStap

	tps.setTraceProcMonitor(&tpm)

	tpm.tpf = &tps

	return &tpm
}

func (t *TraceProcMonitor) Stop() {
	t.tpf.Stop()
}

func (t *TraceProcMonitor) init() {
	t.processMapLock.Lock()
	defer t.processMapLock.Unlock()

	t.childPids = make(map[uint32]struct{})

	t.processMap = make(map[traceProcIndex]*TraceProc)
}

func (t *TraceProcMonitor) Start() {
	t.tpf.Start()
}
