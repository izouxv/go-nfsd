package fs

import (
	"github.com/tchajed/goose/machine/disk"

	"github.com/mit-pdos/goose-nfsd/buf"
)

const (
	NBITBLOCK    uint64 = disk.BlockSize * 8
	INODEBLK     uint64 = disk.BlockSize / INODESZ
	NINODEBITMAP uint64 = 1

	INODESZ uint64 = 64 // on-disk size

	HDRMETA  = uint64(2 * 8) // space for head and tail
	HDRADDRS = (disk.BlockSize - HDRMETA) / 8
	LOGSIZE  = HDRADDRS + 1 // 1 for log header
)

type Inum uint64

type FsSuper struct {
	Size         uint64
	nLog         uint64 // including commit block
	NBlockBitmap uint64
	NInodeBitmap uint64
	nInodeBlk    uint64
	Maxaddr      uint64
}

func MkFsSuper() *FsSuper {
	sz := uint64(10 * 10000)
	nblockbitmap := (sz / NBITBLOCK) + 1
	disk.Init(disk.NewMemDisk(sz))
	return &FsSuper{
		Size:         sz,
		nLog:         LOGSIZE,
		NBlockBitmap: nblockbitmap,
		NInodeBitmap: NINODEBITMAP,
		nInodeBlk:    (NINODEBITMAP * NBITBLOCK * INODESZ) / disk.BlockSize,
		Maxaddr:      sz}
}

func (fs *FsSuper) BitmapBlockStart() uint64 {
	return fs.nLog
}

func (fs *FsSuper) BitmapInodeStart() uint64 {
	return fs.BitmapBlockStart() + fs.NBlockBitmap
}

func (fs *FsSuper) InodeStart() uint64 {
	return fs.BitmapInodeStart() + fs.NInodeBitmap
}

func (fs *FsSuper) DataStart() uint64 {
	return fs.InodeStart() + fs.nInodeBlk
}

func (fs *FsSuper) Block2addr(blkno uint64) buf.Addr {
	return buf.MkAddr(blkno, 0, NBITBLOCK)
}

func (fs *FsSuper) NInode() Inum {
	return Inum(fs.nInodeBlk * INODEBLK)
}

func (fs *FsSuper) Inum2Addr(inum Inum) buf.Addr {
	return buf.MkAddr(fs.InodeStart()+uint64(inum)/INODEBLK, (uint64(inum)%INODEBLK)*INODESZ*8, INODESZ*8)
}
