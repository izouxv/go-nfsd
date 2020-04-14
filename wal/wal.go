package wal

import (
	"sync"

	"github.com/tchajed/goose/machine/disk"

	"github.com/mit-pdos/goose-nfsd/common"
	"github.com/mit-pdos/goose-nfsd/util"
)

func (l *Walog) recover() {
	util.DPrintf(1, "recover %d %d\n", l.st.memStart, l.st.diskEnd)
	for i, buf := range l.st.memLog {
		l.st.memLogMap[buf.Addr] = l.st.memStart + LogPosition(i)
	}
}

func mkLog(disk disk.Disk) *Walog {
	circ, start, end, memLog := recoverCircular(disk)
	ml := new(sync.Mutex)
	st := &WalogState{
		memLog:      memLog,
		memStart:    start,
		diskEnd:     end,
		nextDiskEnd: end,
		memLogMap:   make(map[common.Bnum]LogPosition),
	}
	l := &Walog{
		d:           disk,
		circ:        circ,
		memLock:     ml,
		st:          st,
		condLogger:  sync.NewCond(ml),
		condInstall: sync.NewCond(ml),
		shutdown:    false,
		nthread:     0,
		condShut:    sync.NewCond(ml),
	}
	util.DPrintf(1, "mkLog: size %d\n", LOGSZ)
	l.recover()
	return l
}

func (l *Walog) startBackgroundThreads() {
	go func() { l.logger() }()
	go func() { l.installer() }()
}

func MkLog(disk disk.Disk) *Walog {
	l := mkLog(disk)
	l.startBackgroundThreads()
	return l
}

// memWrite writes out bufs to the in-memory log
//
// Absorbs writes in in-memory transactions (avoiding those that might be in
// the process of being logged or installed).
//
// Assumes caller holds memLock
func (st *WalogState) memWrite(bufs []Update) {
	var pos = st.memStart + LogPosition(len(st.memLog))
	for _, buf := range bufs {
		// remember most recent position for Blkno
		oldpos, ok := st.memLogMap[buf.Addr]
		if ok && oldpos >= st.nextDiskEnd {
			util.DPrintf(5, "memWrite: absorb %d pos %d old %d\n",
				buf.Addr, pos, oldpos)
			// the ownership of this part of the memLog is complicated; maybe the
			// logger and installer don't ever take ownership of it, which is why
			// it's safe to write here?
			st.memLog[oldpos-st.memStart] = buf
			// note that pos does not need to be incremented
		} else {
			if ok {
				util.DPrintf(5, "memLogMap: replace %d pos %d old %d\n",
					buf.Addr, pos, oldpos)
			} else {
				util.DPrintf(5, "memLogMap: add %d pos %d\n",
					buf.Addr, pos)
			}
			st.memLog = append(st.memLog, buf)
			st.memLogMap[buf.Addr] = pos
			pos += 1
		}
	}
	// l.condLogger.Broadcast()
}

// Assumes caller holds memLock
func (st *WalogState) doMemAppend(bufs []Update) LogPosition {
	st.memWrite(bufs)
	txn := st.memStart + LogPosition(len(st.memLog))
	return txn
}

//
//  For clients of WAL
//

func copyUpdateBlock(u Update) disk.Block {
	blk := make([]byte, disk.BlockSize)
	copy(blk, u.Block)
	return blk
}

// readMem implements ReadMem, assuming memLock is held
func (l *Walog) readMem(blkno common.Bnum) (disk.Block, bool) {
	pos, ok := l.st.memLogMap[blkno]
	if ok {
		util.DPrintf(5, "read memLogMap: read %d pos %d\n", blkno, pos)
		u := l.st.memLog[pos-l.st.memStart]
		blk := copyUpdateBlock(u)
		return blk, true
	}
	return nil, false
}

// Read from only the in-memory cached state (the unstable and logged parts of
// the wal).
func (l *Walog) ReadMem(blkno common.Bnum) (disk.Block, bool) {
	l.memLock.Lock()
	blk, ok := l.readMem(blkno)
	l.memLock.Unlock()
	return blk, ok
}

// Read from only the installed state (a subset of durable state).
func (l *Walog) ReadInstalled(blkno common.Bnum) disk.Block {
	return l.d.Read(blkno)
}

// Read reads from the latest memory state, but does so in a
// difficult-to-linearize way (specifically, it is future-dependent when to
// linearize between the l.memLog.Unlock() and the eventual disk read, due to
// potential concurrent cache or disk writes).
func (l *Walog) Read(blkno common.Bnum) disk.Block {
	blk, ok := l.ReadMem(blkno)
	if ok {
		return blk
	}
	return l.ReadInstalled(blkno)
}

// Append to in-memory log.
//
// On success returns the txn for this append.
//
// On failure guaranteed to be idempotent (failure can occur either due to bufs
// exceeding the size of the log or in principle due to overflowing 2^64 writes)
func (l *Walog) MemAppend(bufs []Update) (LogPosition, bool) {
	if uint64(len(bufs)) > LOGSZ {
		return 0, false
	}

	var txn LogPosition = 0
	var ok = true
	l.memLock.Lock()
	for {
		if util.SumOverflows(uint64(l.st.memStart), uint64(len(bufs))) {
			ok = false
			break
		}
		// TODO: relate this calculation to the circular log free space
		memEnd := LogPosition(uint64(l.st.memStart) + uint64(len(l.st.memLog)))
		memSize := uint64(memEnd) - uint64(l.st.diskEnd)
		if memSize+uint64(len(bufs)) > LOGSZ {
			util.DPrintf(5, "memAppend: log is full; try again")
			// commit everything, stable and unstable trans
			l.st.nextDiskEnd = l.st.memStart + LogPosition(len(l.st.memLog))
			l.condLogger.Broadcast()
			l.condLogger.Wait()
			continue
		}
		txn = l.st.doMemAppend(bufs)
		break
	}
	l.memLock.Unlock()
	return txn, ok
}

// Flush flushes a transaction (and all preceding transactions)
//
// The implementation waits until the logger has appended in-memory log up to
// txn to on-disk log.
func (l *Walog) Flush(txn LogPosition) {
	util.DPrintf(1, "Flush: commit till txn %d\n", txn)
	l.memLock.Lock()
	l.condLogger.Broadcast()
	if txn > l.st.nextDiskEnd {
		// a concurrent transaction may already committed beyond txn
		l.st.nextDiskEnd = l.st.memStart + LogPosition(len(l.st.memLog))
	}
	for {
		if txn <= l.st.diskEnd {
			break
		}
		l.condLogger.Wait()
		continue // TODO: without this the loop gooses incorrectly
	}
	l.memLock.Unlock()
}

// Shutdown logger and installer
func (l *Walog) Shutdown() {
	util.DPrintf(1, "shutdown wal\n")
	l.memLock.Lock()
	l.shutdown = true
	l.condLogger.Broadcast()
	l.condInstall.Broadcast()
	for l.nthread > 0 {
		util.DPrintf(1, "wait for logger/installer")
		l.condShut.Wait()
	}
	l.memLock.Unlock()
	util.DPrintf(1, "wal done\n")
}
