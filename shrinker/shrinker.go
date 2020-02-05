package shrinker

import (
	"sync"

	"github.com/mit-pdos/goose-nfsd/common"
	"github.com/mit-pdos/goose-nfsd/fstxn"
	"github.com/mit-pdos/goose-nfsd/util"
)

type ShrinkerSt struct {
	mu       *sync.Mutex
	condShut *sync.Cond
	nthread  uint32
	fsstate  *fstxn.FsState
}

func MkShrinkerSt(st *fstxn.FsState) *ShrinkerSt {
	mu := new(sync.Mutex)
	shrinkst := &ShrinkerSt{
		mu:       mu,
		condShut: sync.NewCond(mu),
		nthread:  0,
		fsstate:  st,
	}
	return shrinkst
}

// If caller changes file size and shrinking is in progress (because
// an earlier call truncated the file), then help/wait with/for
// shrinking.  Also, called by shrinker.
func (shrinkst *ShrinkerSt) DoShrink(inum common.Inum) bool {
	var more = true
	var ok = true
	for more {
		op := fstxn.Begin(shrinkst.fsstate)
		ip := op.GetInodeInumFree(inum)
		if ip == nil {
			panic("shrink")
		}
		util.DPrintf(1, "%d: doShrink %v\n", op.Atxn.Id(), ip.Inum)
		more = ip.Shrink(op.Atxn)
		ok = op.Commit()
		if !ok {
			break
		}
	}
	return ok
}

func (shrinker *ShrinkerSt) Shutdown() {
	shrinker.mu.Lock()
	for shrinker.nthread > 0 {
		util.DPrintf(1, "ShutdownNfs: wait %d\n", shrinker.nthread)
		shrinker.condShut.Wait()
	}
	shrinker.mu.Unlock()
}

// for large files, start a separate thread
func (shrinkst *ShrinkerSt) StartShrinker(inum common.Inum) {
	util.DPrintf(1, "start shrink thread\n")
	shrinkst.mu.Lock()
	shrinkst.nthread = shrinkst.nthread + 1
	shrinkst.mu.Unlock()
	go func() { shrinkst.shrinker(inum) }()
}

func (shrinkst *ShrinkerSt) shrinker(inum common.Inum) {
	ok := shrinkst.DoShrink(inum)
	if !ok {
		panic("shrink")
	}
	util.DPrintf(1, "Shrinker: done shrinking # %d\n", inum)
	shrinkst.mu.Lock()
	shrinkst.nthread = shrinkst.nthread - 1
	shrinkst.condShut.Signal()
	shrinkst.mu.Unlock()
}