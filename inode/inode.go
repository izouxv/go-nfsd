package inode

import (
	"fmt"
	"time"

	"github.com/tchajed/goose/machine"
	"github.com/tchajed/goose/machine/disk"

	"github.com/mit-pdos/goose-nfsd/buf"
	"github.com/mit-pdos/goose-nfsd/dcache"
	"github.com/mit-pdos/goose-nfsd/fh"
	"github.com/mit-pdos/goose-nfsd/fs"
	"github.com/mit-pdos/goose-nfsd/fstxn"
	"github.com/mit-pdos/goose-nfsd/marshal"
	"github.com/mit-pdos/goose-nfsd/nfstypes"
	"github.com/mit-pdos/goose-nfsd/util"
)

const NF3FREE nfstypes.Ftype3 = 0

const (
	NBLKINO   uint64 = 3 // # blk in an inode's blks array
	NDIRECT   uint64 = NBLKINO - 2
	INDIRECT  uint64 = NBLKINO - 2
	DINDIRECT uint64 = NBLKINO - 1
	NBLKBLK   uint64 = disk.BlockSize / 8 // # blkno per block
	NINDLEVEL uint64 = 2                  // # levels of indirection
)

type Inode struct {
	// in-memory info:
	Inum   fs.Inum
	Dcache *dcache.Dcache

	// the on-disk inode:
	Kind  nfstypes.Ftype3
	Nlink uint32
	Gen   uint64
	Size  uint64
	Atime nfstypes.Nfstime3
	Mtime nfstypes.Nfstime3
	blks  []uint64
}

func NfstimeNow() nfstypes.Nfstime3 {
	now := time.Now()
	t := nfstypes.Nfstime3{
		Seconds:  nfstypes.Uint32(now.Unix()),
		Nseconds: nfstypes.Uint32(now.Nanosecond()),
	}
	return t
}

func MkNullInode() *Inode {
	return &Inode{
		Inum:  fs.NULLINUM,
		Kind:  nfstypes.NF3DIR,
		Nlink: uint32(1),
		Gen:   uint64(0),
		Size:  uint64(0),
		Atime: NfstimeNow(),
		Mtime: NfstimeNow(),
		blks:  make([]uint64, NBLKINO),
	}
}

func MkRootInode() *Inode {
	return &Inode{
		Inum:  fs.ROOTINUM,
		Kind:  nfstypes.NF3DIR,
		Nlink: uint32(1),
		Gen:   uint64(0),
		Size:  uint64(0),
		Atime: NfstimeNow(),
		Mtime: NfstimeNow(),
		blks:  make([]uint64, NBLKINO),
	}
}

func (ip *Inode) initInode(inum fs.Inum, kind nfstypes.Ftype3) {
	util.DPrintf(1, "initInode: inode # %d\n", inum)
	ip.Inum = inum
	ip.Kind = kind
	ip.Nlink = 1
	ip.Gen = ip.Gen + 1
	ip.Atime = NfstimeNow()
	ip.Mtime = NfstimeNow()
}

func (ip *Inode) String() string {
	return fmt.Sprintf("# %d k %d n %d g %d sz %d %v", ip.Inum, ip.Kind, ip.Nlink, ip.Gen, ip.Size, ip.blks)
}

func (ip *Inode) MkFattr() nfstypes.Fattr3 {
	return nfstypes.Fattr3{
		Ftype: ip.Kind,
		Mode:  0777,
		Nlink: 1,
		Uid:   nfstypes.Uid3(0),
		Gid:   nfstypes.Gid3(0),
		Size:  nfstypes.Size3(ip.Size),
		Used:  nfstypes.Size3(ip.Size),
		Rdev: nfstypes.Specdata3{Specdata1: nfstypes.Uint32(0),
			Specdata2: nfstypes.Uint32(0)},
		Fsid:   nfstypes.Uint64(0),
		Fileid: nfstypes.Fileid3(ip.Inum),
		Atime:  ip.Atime,
		Mtime:  ip.Mtime,
		Ctime: nfstypes.Nfstime3{Seconds: nfstypes.Uint32(0),
			Nseconds: nfstypes.Uint32(0)},
	}
}

func (ip *Inode) Encode() []byte {
	d := make([]byte, fs.INODESZ)
	enc := marshal.NewEnc(d)
	enc.PutInt32(uint32(ip.Kind))
	enc.PutInt32(ip.Nlink)
	enc.PutInt(ip.Gen)
	enc.PutInt(ip.Size)
	enc.PutInt32(uint32(ip.Atime.Seconds))
	enc.PutInt32(uint32(ip.Atime.Nseconds))
	enc.PutInt32(uint32(ip.Mtime.Seconds))
	enc.PutInt32(uint32(ip.Mtime.Nseconds))
	enc.PutInts(ip.blks)
	return d
}

func Decode(buf *buf.Buf, inum fs.Inum) *Inode {
	ip := &Inode{
		Inum:  0,
		Kind:  0,
		Nlink: 0,
		Gen:   0,
		Size:  0,
		blks:  nil,
	}
	dec := marshal.NewDec(buf.Blk)
	ip.Inum = inum
	ip.Kind = nfstypes.Ftype3(dec.GetInt32())
	ip.Nlink = dec.GetInt32()
	ip.Gen = dec.GetInt()
	ip.Size = dec.GetInt()
	ip.Atime.Seconds = nfstypes.Uint32(dec.GetInt32())
	ip.Atime.Nseconds = nfstypes.Uint32(dec.GetInt32())
	ip.Mtime.Seconds = nfstypes.Uint32(dec.GetInt32())
	ip.Mtime.Nseconds = nfstypes.Uint32(dec.GetInt32())
	ip.blks = dec.GetInts(NBLKINO)
	return ip
}

func pow(level uint64) uint64 {
	if level == 0 {
		return 1
	}
	var p uint64 = NBLKBLK
	for i := uint64(1); i < level; i++ {
		p = p * p
	}
	return p
}

func MaxFileSize() uint64 {
	maxblks := pow(NINDLEVEL)
	return (NDIRECT + maxblks) * disk.BlockSize
}

func GetInodeLocked(op *fstxn.FsTxn, inum fs.Inum) *Inode {
	addr := op.Fs.Inum2Addr(inum)
	op.Acquire(addr)
	cslot := op.LookupSlot(inum)
	if cslot == nil {
		panic("GetInodeLocked")
	}
	if cslot.Obj == nil {
		buf := op.ReadBuf(addr)
		i := Decode(buf, inum)
		util.DPrintf(1, "GetInodeLocked # %v: read inode from disk\n", inum)
		cslot.Obj = i
	}
	i := cslot.Obj.(*Inode)
	util.DPrintf(1, "GetInodeLocked %v\n", i)
	return i
}

func getInodeInumFree(op *fstxn.FsTxn, inum fs.Inum) *Inode {
	ip := GetInodeLocked(op, inum)
	return ip
}

func GetInodeInum(op *fstxn.FsTxn, inum fs.Inum) *Inode {
	ip := getInodeInumFree(op, inum)
	if ip == nil {
		return nil
	}
	if ip.Kind == NF3FREE {
		ip.Put(op)
		ip.ReleaseInode(op)
		return nil
	}
	if ip.Nlink == 0 {
		panic("getInodeInum")
	}
	return ip
}

func GetInode(op *fstxn.FsTxn, fh3 nfstypes.Nfs_fh3) *Inode {
	fh := fh.MakeFh(fh3)
	ip := GetInodeInum(op, fh.Ino)
	if ip == nil {
		return nil
	}
	if ip.Gen != fh.Gen {
		ip.Put(op)
		ip.ReleaseInode(op)
		return nil
	}
	return ip
}

func (ip *Inode) ReleaseInode(op *fstxn.FsTxn) {
	addr := op.Fs.Inum2Addr(ip.Inum)
	op.Release(addr)
}

func (ip *Inode) WriteInode(op *fstxn.FsTxn) {
	if ip.Inum >= op.Fs.NInode() {
		panic("WriteInode")
	}
	d := ip.Encode()
	op.OverWrite(op.Fs.Inum2Addr(ip.Inum), d)
	util.DPrintf(1, "WriteInode %v\n", ip)
}

func AllocInode(op *fstxn.FsTxn, kind nfstypes.Ftype3) (fs.Inum, *Inode) {
	var ip *Inode
	inum := op.AllocINum()
	if inum != fs.NULLINUM {
		ip = GetInodeLocked(op, inum)
		if ip.Kind == NF3FREE {
			ip.initInode(inum, kind)
		} else {
			panic("AllocInode")
		}
		ip.WriteInode(op)
	}
	return inum, ip
}

func (ip *Inode) freeInode(op *fstxn.FsTxn) {
	ip.Kind = NF3FREE
	ip.Gen = ip.Gen + 1
	ip.WriteInode(op)
	op.FreeINum(ip.Inum)
}

func FreeInum(op *fstxn.FsTxn, inum fs.Inum) {
	i := GetInodeLocked(op, inum)
	if i.Kind == NF3FREE {
		panic("freeInode")
	}
	i.freeInode(op)
}

// Done with ip and remove inode if Nlink = 0.
func (ip *Inode) Put(op *fstxn.FsTxn) {
	util.DPrintf(1, "put inode # %d Nlink %d\n", ip.Inum, ip.Nlink)
	// shrinker may put an FREE inode
	if ip.Nlink == 0 && ip.Kind != NF3FREE {
		ip.Resize(op, 0)
		ip.freeInode(op)
	}
	op.FreeSlot(ip.Inum)
}

func putInodes(op *fstxn.FsTxn, inodes []*Inode) {
	for _, ip := range inodes {
		ip.Put(op)
	}
}

// Returns blkno for indirect bn and newroot if root was allocated. If
// blkno is 0, failure.
func (ip *Inode) indbmap(op *fstxn.FsTxn, root uint64, level uint64, off uint64, grow bool) (uint64, uint64) {
	var newroot uint64 = 0
	var blkno uint64 = root

	if blkno == 0 { // no root?
		newroot = op.AllocBlock()
		if newroot == 0 {
			return 0, 0
		}
		blkno = newroot
	}

	if level == 0 { // leaf?
		if root != 0 && grow { // old leaf?
			op.ZeroBlock(blkno)
		}
		return blkno, newroot
	}

	divisor := pow(level - 1)
	o := off / divisor
	bo := o * 8
	ind := off % divisor

	if root != 0 && off == 0 && grow { // old root from previous file?
		op.ZeroBlock(blkno)
	}

	buf := op.ReadBlock(blkno)
	nxtroot := buf.Uint64Get(bo)
	util.DPrintf(1, "%d next root %v level %d\n", blkno, nxtroot, level)
	b, newroot1 := ip.indbmap(op, nxtroot, level-1, ind, grow)
	op.AssertValidBlock(newroot1)
	if newroot1 != 0 {
		buf.Uint64Put(bo, newroot1)
	}
	if b >= op.Fs.Size {
		util.DPrintf(0, "indbmap %v %v\n", b, op.Fs.Size)
		panic("indbmap")
	}
	return b, newroot
}

// Lazily resize file. Bmap allocates/zeros blocks on demand.  Create
// a new thread to free blocks in a separate transaction.
func (ip *Inode) Resize(op *fstxn.FsTxn, sz uint64) {
	util.DPrintf(5, "resize %v to sz %d\n", ip, sz)
	oldsz := ip.Size
	ip.Size = sz
	ip.WriteInode(op)
	if sz < oldsz {
		if ip.smallFileFits(op) {
			bn := util.RoundUp(oldsz, disk.BlockSize)
			ip.shrink(op, bn)
			util.DPrintf(1, "small file delete inside trans\n")
		} else {
			// for large files, start a separate thread
			util.DPrintf(1, "start shrink thread\n")
			shrinkst.mu.Lock()
			shrinkst.nthread = shrinkst.nthread + 1
			shrinkst.mu.Unlock()
			machine.Spawn(func() { shrinker(ip.Inum, oldsz) })
		}
	}
}

// Map logical block number bn to a physical block number, allocating
// blocks if no block exists for bn. Reuse block from previous
// versions of this inode, but zero them.
func (ip *Inode) bmap(op *fstxn.FsTxn, bn uint64) (uint64, bool) {
	var alloc bool = false
	sz := util.RoundUp(ip.Size, disk.BlockSize)
	grow := bn > sz
	if bn < NDIRECT {
		if ip.blks[bn] != 0 && grow {
			op.ZeroBlock(ip.blks[bn])
		}
		if ip.blks[bn] == 0 {
			blkno := op.AllocBlock()
			if blkno == 0 {
				return 0, false
			}
			alloc = true
			ip.blks[bn] = blkno
		}
		return ip.blks[bn], alloc
	} else {
		var off = bn - NDIRECT
		if off < NBLKBLK {
			blkno, newroot := ip.indbmap(op, ip.blks[INDIRECT], 1, off, grow)
			if newroot != 0 {
				ip.blks[INDIRECT] = newroot
			}
			return blkno, newroot != 0
		} else {
			off -= NBLKBLK
			blkno, newroot := ip.indbmap(op, ip.blks[DINDIRECT], 2, off, grow)
			if newroot != 0 {
				ip.blks[DINDIRECT] = newroot
			}
			return blkno, newroot != 0
		}
	}
}

// Returns number of bytes read and eof
func (ip *Inode) Read(op *fstxn.FsTxn, offset uint64, bytesToRead uint64) ([]byte,
	bool) {
	var n uint64 = uint64(0)

	if offset >= ip.Size {
		return nil, true
	}
	var count uint64 = bytesToRead
	if count >= offset+ip.Size {
		count = ip.Size - offset
	}
	util.DPrintf(5, "Read: off %d cnt %d\n", offset, count)
	var data = make([]byte, 0)
	var off = offset
	for boff := off / disk.BlockSize; n < count; boff++ {
		byteoff := off % disk.BlockSize
		nbytes := util.Min(disk.BlockSize-byteoff, count-n)
		blkno, alloc := ip.bmap(op, boff)
		if blkno == 0 {
			return data, false
		}
		if alloc { // fill in a hole
			ip.WriteInode(op)
		}
		buf := op.ReadBlock(blkno)

		for b := uint64(0); b < nbytes; b++ {
			data = append(data, buf.Blk[byteoff+b])
		}
		n += nbytes
		off += nbytes
	}
	util.DPrintf(10, "Read: off %d cnt %d -> %v\n", offset, count, data)
	return data, false
}

// Returns number of bytes written and error
func (ip *Inode) Write(op *fstxn.FsTxn, offset uint64,
	count uint64, dataBuf []byte) (uint64, bool) {
	var cnt uint64 = uint64(0)
	var off uint64 = offset
	var ok bool = true
	var alloc bool = false
	var n = count
	var data = dataBuf

	util.DPrintf(5, "Write: off %d cnt %d\n", offset, count)
	if offset+count > MaxFileSize() {
		return 0, false
	}
	for boff := off / disk.BlockSize; n > uint64(0); boff++ {
		blkno, new := ip.bmap(op, boff)
		if blkno == 0 {
			ok = false
			break
		}
		if new {
			alloc = true
		}
		byteoff := off % disk.BlockSize
		var nbytes = disk.BlockSize - byteoff
		if n < nbytes {
			nbytes = n
		}
		if byteoff == 0 && nbytes == disk.BlockSize { // block overwrite?
			addr := op.Fs.Block2addr(blkno)
			op.Acquire(addr)
			op.OverWrite(addr, data[0:nbytes])
		} else {
			buffer := op.ReadBlock(blkno)
			for b := uint64(0); b < nbytes; b++ {
				buffer.Blk[byteoff+b] = data[b]
			}
			buffer.SetDirty()
		}
		n -= nbytes
		data = data[nbytes:]
		off += nbytes
		cnt += nbytes
	}
	util.DPrintf(1, "Write: off %d cnt %d size %d\n", offset, cnt, ip.Size)
	if alloc || cnt > 0 {
		if offset+cnt > ip.Size {
			ip.Size = offset + cnt
		}
		ip.WriteInode(op)
		return cnt, true
	}
	return cnt, ok
}

func (ip *Inode) DecLink(op *fstxn.FsTxn) {
	ip.Nlink = ip.Nlink - 1
	ip.WriteInode(op)
}
