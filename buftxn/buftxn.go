package buftxn

import (
	"github.com/tchajed/goose/machine/disk"

	"github.com/mit-pdos/goose-nfsd/buf"
	"github.com/mit-pdos/goose-nfsd/txn"
)

type BufTxn struct {
	txn  *txn.Txn
	bufs *buf.BufMap // map of bufs read/written by trans
	id   txn.TransId
}

func Begin(txn *txn.Txn) *BufTxn {
	trans := &BufTxn{
		txn:  txn,
		bufs: buf.MkBufMap(),
		id:   txn.GetTransId(),
	}
	return trans
}

func (buftxn *BufTxn) ReadBufLocked(addr buf.Addr) *buf.Buf {
	first := buftxn.txn.Lock(addr, buftxn.id)
	if first {
		buf := buf.MkBufData(addr)
		buftxn.txn.Load(buf)
		buftxn.bufs.Insert(buf)
	}
	b := buftxn.bufs.Lookup(addr)
	return b
}

func (buftxn *BufTxn) Release(addr buf.Addr) {
	buftxn.bufs.Del(addr)
	buftxn.txn.Release(addr, buftxn.id)
}

func (buftxn *BufTxn) NDirty() uint64 {
	return buftxn.bufs.Ndirty()
}

func (buftxn *BufTxn) LogSz() uint64 {
	return buftxn.txn.LogSz()
}

func (buftxn *BufTxn) LogSzBytes() uint64 {
	return buftxn.txn.LogSz() * disk.BlockSize
}

// Commit bufs of this transaction
func (buftxn *BufTxn) CommitWait(wait bool, abort bool) bool {
	return buftxn.txn.CommitWait(buftxn.bufs.Bufs(), wait, abort, buftxn.id)
}

func (buftxn *BufTxn) Flush() bool {
	return buftxn.txn.Flush(buftxn.id)
}