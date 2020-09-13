package simple

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	// "github.com/stretchr/testify/require"
	"github.com/tchajed/goose/machine/disk"

	"github.com/mit-pdos/goose-nfsd/common"
	"github.com/mit-pdos/goose-nfsd/fh"
	"github.com/mit-pdos/goose-nfsd/nfstypes"
	"github.com/mit-pdos/goose-nfsd/util"

	"github.com/stretchr/testify/assert"

	"testing"
)

var quiet = flag.Bool("quiet", false, "disable logging")

const DISKSZ uint64 = 10 * 1000

func checkFlags() {
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
}

type NfsClient struct {
	srv *Nfs
}

func MkNfsClient() *NfsClient {
	sz := uint64(100 * 1024)
	r := rand.Uint64()
	tmpdir := "/dev/shm"
	f, err := os.Stat(tmpdir)
	if !(err == nil && f.IsDir()) {
		tmpdir = os.TempDir()
	}
	n := filepath.Join(tmpdir, "goose"+strconv.FormatUint(r, 16)+".img")

	var d disk.Disk
	util.DPrintf(1, "MakeNfs: creating file disk at %s", n)
	d, err = disk.NewFileDisk(n, sz)
	if err != nil {
		panic(fmt.Errorf("could not create file disk: %v", err))
	}

	return &NfsClient{
		srv: MakeNfs(d),
	}
}

func (clnt *NfsClient) ReadDirOp(dir nfstypes.Nfs_fh3, cnt uint64) nfstypes.READDIR3res {
	args := nfstypes.READDIR3args{Dir: dir, Count: nfstypes.Count3(100)}
	reply := clnt.srv.NFSPROC3_READDIR(args)
	return reply
}

func (clnt *NfsClient) WriteOp(fh nfstypes.Nfs_fh3, off uint64, data []byte, how nfstypes.Stable_how) *nfstypes.WRITE3res {
	args := nfstypes.WRITE3args{
		File:   fh,
		Offset: nfstypes.Offset3(off),
		Count:  nfstypes.Count3(len(data)),
		Stable: how,
		Data:   data}
	reply := clnt.srv.NFSPROC3_WRITE(args)
	return &reply
}

func mkdata(sz uint64) []byte {
	data := make([]byte, sz)
	for i := range data {
		data[i] = byte(i % 128)
	}
	return data
}

type TestState struct {
	t    *testing.T
	clnt *NfsClient
}

func newTest(t *testing.T) *TestState {
	checkFlags()
	fmt.Printf("%s\n", t.Name())
	ts := &TestState{t: t}
	ts.clnt = MkNfsClient()
	return ts
}

func (ts *TestState) ReadDir() nfstypes.Dirlist3 {
	reply := ts.clnt.ReadDirOp(fh.MkRootFh3(), disk.BlockSize)
	assert.Equal(ts.t, reply.Status, nfstypes.NFS3_OK)
	return reply.Resok.Reply
}

func (ts *TestState) Write(fh nfstypes.Nfs_fh3, data []byte, how nfstypes.Stable_how) {
	reply := ts.clnt.WriteOp(fh, uint64(0), data, how)
	assert.Equal(ts.t, nfstypes.NFS3_OK, reply.Status)
	assert.Equal(ts.t, nfstypes.Count3(len(data)), reply.Resok.Count)
}

func TestReadDir(t *testing.T) {
	checkFlags()
	ts := newTest(t)

	dl3 := ts.ReadDir()
	ne3 := dl3.Entries
	n := 2
	for ne3 != nil {
		assert.Equal(t, nfstypes.Fileid3(n), ne3.Fileid)
		n++
		ne3 = ne3.Nextentry
	}
}

func TestFile(t *testing.T) {
	ts := newTest(t)

	fh := fh.Fh{common.Inum(2), uint64(0)}
	data := mkdata(4096)
	ts.Write(fh.MakeFh3(), data, nfstypes.FILE_SYNC)
}