(* autogenerated from wal *)
From Perennial.go_lang Require Import prelude.

(* disk FFI *)
From Perennial.go_lang Require Import ffi.disk_prelude.

(* 00util.go *)

Definition Debug : expr := #0.

Definition RoundUp: val :=
  λ: "n" "sz",
    "n" + "sz" - #1 `quot` "sz".

Definition Min: val :=
  λ: "n" "m",
    (if: "n" < "m"
    then "n"
    else "m").

Definition xxcopy: val :=
  λ: "dst" "src",
    let: "dlen" := slice.len "dst" in
    let: "slen" := slice.len "src" in
    let: "copylen" := Min "dlen" "slen" in
    let: "i" := ref #0 in
    (for: (!"i" < "copylen"); ("i" <- !"i" + #1) :=
      SliceSet "dst" !"i" (SliceGet "src" !"i");;
      Continue).

(* enc_dec.go *)

Module enc.
  Definition S := struct.decl [
    "b" :: disk.blockT;
    "off" :: uint64T
  ].
  Definition T: ty := struct.t S.
  Definition Ptr: ty := struct.ptrT S.
  Section fields.
    Context `{ext_ty: ext_types}.
    Definition get := struct.get S.
  End fields.
End enc.

Definition NewEnc: val :=
  λ: "blk",
    struct.new enc.S [
      "b" ::= "blk";
      "off" ::= #0
    ].

Definition enc__PutInt32: val :=
  λ: "enc" "x",
    let: "off" := struct.loadF enc.S "off" "enc" in
    UInt32Put (SliceSubslice (struct.loadF enc.S "b" "enc") "off" ("off" + #4)) "x";;
    struct.storeF enc.S "off" "enc" (struct.loadF enc.S "off" "enc" + #4).

Definition enc__PutInt: val :=
  λ: "enc" "x",
    let: "off" := struct.loadF enc.S "off" "enc" in
    UInt64Put (SliceSubslice (struct.loadF enc.S "b" "enc") "off" ("off" + #8)) "x";;
    struct.storeF enc.S "off" "enc" (struct.loadF enc.S "off" "enc" + #8).

Definition enc__PutInts: val :=
  λ: "enc" "xs",
    ForSlice <> "x" "xs"
      (enc__PutInt "enc" "x").

Module dec.
  Definition S := struct.decl [
    "b" :: disk.blockT;
    "off" :: uint64T
  ].
  Definition T: ty := struct.t S.
  Definition Ptr: ty := struct.ptrT S.
  Section fields.
    Context `{ext_ty: ext_types}.
    Definition get := struct.get S.
  End fields.
End dec.

Definition NewDec: val :=
  λ: "b",
    struct.new dec.S [
      "b" ::= "b";
      "off" ::= #0
    ].

Definition dec__GetInt: val :=
  λ: "dec",
    let: "off" := struct.loadF dec.S "off" "dec" in
    let: "x" := UInt64Get (SliceSubslice (struct.loadF dec.S "b" "dec") "off" ("off" + #8)) in
    struct.storeF dec.S "off" "dec" (struct.loadF dec.S "off" "dec" + #8);;
    "x".

Definition dec__GetInt32: val :=
  λ: "dec",
    let: "off" := struct.loadF dec.S "off" "dec" in
    let: "x" := UInt32Get (SliceSubslice (struct.loadF dec.S "b" "dec") "off" ("off" + #4)) in
    struct.storeF dec.S "off" "dec" (struct.loadF dec.S "off" "dec" + #4);;
    "x".

Definition dec__GetInts: val :=
  λ: "dec" "len",
    let: "xs" := NewSlice uint64T "len" in
    ForSlice "i" <> "xs"
      (SliceSet "xs" "i" (dec__GetInt "dec"));;
    "xs".

Definition PutBytes: val :=
  λ: "d" "b",
    let: "i" := ref #0 in
    (for: (!"i" < slice.len "b"); ("i" <- !"i" + #1) :=
      SliceSet "d" !"i" (SliceGet "b" !"i");;
      Continue).

(* fs.go *)

Definition NBITBLOCK : expr := disk.BlockSize * #8.

Definition INODEBLK : expr := disk.BlockSize `quot` "INODESZ".

Definition NINODEBITMAP : expr := #1.

(* on-disk size *)
Definition INODESZ : expr := #64.

(* space for the end position *)
Definition HDRMETA : expr := #8.

Definition HDRADDRS : expr := disk.BlockSize - HDRMETA `quot` #8.

(* 2 for log header *)
Definition LOGSIZE : expr := HDRADDRS + #2.

Definition Inum: ty := uint64T.

Definition NULLINUM : expr := #0.

Definition ROOTINUM : expr := #1.

Module FsSuper.
  Definition S := struct.decl [
    "Size" :: uint64T;
    "nLog" :: uint64T;
    "NBlockBitmap" :: uint64T;
    "NInodeBitmap" :: uint64T;
    "nInodeBlk" :: uint64T;
    "Maxaddr" :: uint64T
  ].
  Definition T: ty := struct.t S.
  Definition Ptr: ty := struct.ptrT S.
  Section fields.
    Context `{ext_ty: ext_types}.
    Definition get := struct.get S.
  End fields.
End FsSuper.

Definition FsSuper__BitmapBlockStart: val :=
  λ: "fs",
    struct.loadF FsSuper.S "nLog" "fs".

Definition FsSuper__BitmapInodeStart: val :=
  λ: "fs",
    FsSuper__BitmapBlockStart "fs" + struct.loadF FsSuper.S "NBlockBitmap" "fs".

Definition FsSuper__InodeStart: val :=
  λ: "fs",
    FsSuper__BitmapInodeStart "fs" + struct.loadF FsSuper.S "NInodeBitmap" "fs".

Definition FsSuper__DataStart: val :=
  λ: "fs",
    FsSuper__InodeStart "fs" + struct.loadF FsSuper.S "nInodeBlk" "fs".

(* wal.go *)

Definition LOGHDR : expr := #0.

Definition LOGHDR2 : expr := #1.

Definition LOGSTART : expr := #2.

Module BlockData.
  Definition S := struct.decl [
    "Blocknum" :: uint64T;
    "Data" :: disk.blockT
  ].
  Definition T: ty := struct.t S.
  Definition Ptr: ty := struct.ptrT S.
  Section fields.
    Context `{ext_ty: ext_types}.
    Definition get := struct.get S.
  End fields.
End BlockData.

Module Walog.
  Definition S := struct.decl [
    "memLock" :: lockRefT;
    "condLogger" :: condvarRefT;
    "condInstall" :: condvarRefT;
    "memLog" :: slice.T BlockData.T;
    "memStart" :: uint64T;
    "diskEnd" :: uint64T;
    "shutdown" :: boolT
  ].
  Definition T: ty := struct.t S.
  Definition Ptr: ty := struct.ptrT S.
  Section fields.
    Context `{ext_ty: ext_types}.
    Definition get := struct.get S.
  End fields.
End Walog.

Module hdr.
  (* On-disk header in the first block of the log *)
  Definition S := struct.decl [
    "end" :: uint64T;
    "addrs" :: slice.T uint64T
  ].
  Definition T: ty := struct.t S.
  Definition Ptr: ty := struct.ptrT S.
  Section fields.
    Context `{ext_ty: ext_types}.
    Definition get := struct.get S.
  End fields.
End hdr.

Definition decodeHdr: val :=
  λ: "blk",
    let: "h" := struct.new hdr.S [
      "end" ::= #0;
      "addrs" ::= slice.nil
    ] in
    let: "dec" := NewDec "blk" in
    struct.storeF hdr.S "end" "h" (dec__GetInt "dec");;
    struct.storeF hdr.S "addrs" "h" (dec__GetInts "dec" HDRADDRS);;
    "h".

Definition encodeHdr: val :=
  λ: "h" "blk",
    let: "enc" := NewEnc "blk" in
    enc__PutInt "enc" (hdr.get "end" "h");;
    enc__PutInts "enc" (hdr.get "addrs" "h").

Module hdr2.
  (* On-disk header in the second block of the log *)
  Definition S := struct.decl [
    "start" :: uint64T
  ].
  Definition T: ty := struct.t S.
  Definition Ptr: ty := struct.ptrT S.
  Section fields.
    Context `{ext_ty: ext_types}.
    Definition get := struct.get S.
  End fields.
End hdr2.

Definition decodeHdr2: val :=
  λ: "blk",
    let: "h" := struct.new hdr2.S [
      "start" ::= #0
    ] in
    let: "dec" := NewDec "blk" in
    struct.storeF hdr2.S "start" "h" (dec__GetInt "dec");;
    "h".

Definition encodeHdr2: val :=
  λ: "h" "blk",
    let: "enc" := NewEnc "blk" in
    enc__PutInt "enc" (hdr2.get "start" "h").

Definition Walog__writeHdr: val :=
  λ: "l" "h",
    let: "blk" := NewSlice byteT disk.BlockSize in
    encodeHdr (struct.load hdr.S "h") "blk";;
    disk.Write LOGHDR "blk".

Definition Walog__readHdr: val :=
  λ: "l",
    let: "blk" := disk.Read LOGHDR in
    let: "h" := decodeHdr "blk" in
    "h".

Definition Walog__writeHdr2: val :=
  λ: "l" "h",
    let: "blk" := NewSlice byteT disk.BlockSize in
    encodeHdr2 (struct.load hdr2.S "h") "blk";;
    disk.Write LOGHDR2 "blk".

Definition Walog__readHdr2: val :=
  λ: "l",
    let: "blk" := disk.Read LOGHDR2 in
    let: "h" := decodeHdr2 "blk" in
    "h".

Definition Walog__installBlocks: val :=
  λ: "l" "bufs",
    let: "n" := slice.len "bufs" in
    let: "i" := ref #0 in
    (for: (!"i" < "n"); ("i" <- !"i" + #1) :=
      let: "blkno" := BlockData.get "Blocknum" (SliceGet "bufs" !"i") in
      let: "blk" := BlockData.get "Data" (SliceGet "bufs" !"i") in
      disk.Write "blkno" "blk";;
      Continue).

(* Installer holds logLock
   XXX absorp *)
Definition Walog__logInstall: val :=
  λ: "l",
    let: "installEnd" := struct.loadF Walog.S "diskEnd" "l" in
    let: "bufs" := SliceTake (struct.loadF Walog.S "memLog" "l") ("installEnd" - struct.loadF Walog.S "memStart" "l") in
    (if: slice.len "bufs" = #0
    then (#0, "installEnd")
    else
      lock.release (struct.loadF Walog.S "memLock" "l");;
      Walog__installBlocks "l" "bufs";;
      let: "h" := struct.new hdr2.S [
        "start" ::= "installEnd"
      ] in
      Walog__writeHdr2 "l" "h";;
      lock.acquire (struct.loadF Walog.S "memLock" "l");;
      (if: "installEnd" < struct.loadF Walog.S "memStart" "l"
      then
        Panic "logInstall";;
        #()
      else #());;
      struct.storeF Walog.S "memLog" "l" (SliceSkip (struct.loadF Walog.S "memLog" "l") ("installEnd" - struct.loadF Walog.S "memStart" "l"));;
      struct.storeF Walog.S "memStart" "l" "installEnd";;
      (slice.len "bufs", "installEnd")).

Definition Walog__installer: val :=
  λ: "l",
    lock.acquire (struct.loadF Walog.S "memLock" "l");;
    Skip;;
    (for: (~ (struct.loadF Walog.S "shutdown" "l")); (Skip) :=
      Walog__logInstall "l";;
      Continue);;
    lock.release (struct.loadF Walog.S "memLock" "l").

Definition Walog__LogSz: val :=
  λ: "l",
    HDRADDRS.

Definition Walog__logBlocks: val :=
  λ: "l" "memend" "memstart" "diskend" "bufs",
    let: "pos" := ref "diskend" in
    (for: (!"pos" < "memend"); ("pos" <- !"pos" + #1) :=
      let: "buf" := SliceGet "bufs" (!"pos" - "diskend") in
      let: "blk" := BlockData.get "Data" "buf" in
      disk.Write (LOGSTART + !"pos" `rem` Walog__LogSz "l") "blk";;
      Continue).

(* Logger holds logLock *)
Definition Walog__logAppend: val :=
  λ: "l",
    let: "memstart" := struct.loadF Walog.S "memStart" "l" in
    let: "memlog" := struct.loadF Walog.S "memLog" "l" in
    let: "memend" := "memstart" + slice.len "memlog" in
    let: "diskend" := struct.loadF Walog.S "diskEnd" "l" in
    let: "newbufs" := SliceSkip "memlog" ("diskend" - "memstart") in
    (if: slice.len "newbufs" = #0
    then #()
    else
      lock.release (struct.loadF Walog.S "memLock" "l");;
      Walog__logBlocks "l" "memend" "memstart" "diskend" "newbufs";;
      let: "addrs" := NewSlice uint64T (Walog__LogSz "l") in
      let: "i" := ref #0 in
      (for: (!"i" < slice.len "memlog"); ("i" <- !"i" + #1) :=
        let: "pos" := "memstart" + !"i" in
        SliceSet "addrs" ("pos" `rem` Walog__LogSz "l") (BlockData.get "Blocknum" (SliceGet "memlog" !"i"));;
        Continue);;
      let: "newh" := struct.new hdr.S [
        "end" ::= "memend";
        "addrs" ::= "addrs"
      ] in
      Walog__writeHdr "l" "newh";;
      lock.acquire (struct.loadF Walog.S "memLock" "l");;
      struct.storeF Walog.S "diskEnd" "l" "memend").

Definition Walog__logger: val :=
  λ: "l",
    lock.acquire (struct.loadF Walog.S "memLock" "l");;
    Skip;;
    (for: (~ (struct.loadF Walog.S "shutdown" "l")); (Skip) :=
      Walog__logAppend "l";;
      Continue);;
    lock.release (struct.loadF Walog.S "memLock" "l").

Definition Walog__recover: val :=
  λ: "l",
    let: "h" := Walog__readHdr "l" in
    let: "h2" := Walog__readHdr2 "l" in
    struct.storeF Walog.S "memStart" "l" (struct.loadF hdr2.S "start" "h2");;
    struct.storeF Walog.S "diskEnd" "l" (struct.loadF hdr.S "end" "h");;
    let: "pos" := ref (struct.loadF hdr2.S "start" "h2") in
    (for: (!"pos" < struct.loadF hdr.S "end" "h"); ("pos" <- !"pos" + #1) :=
      let: "addr" := SliceGet (struct.loadF hdr.S "addrs" "h") (!"pos" `rem` Walog__LogSz "l") in
      let: "blk" := disk.Read (LOGSTART + !"pos" `rem` Walog__LogSz "l") in
      let: "b" := struct.mk BlockData.S [
        "Blocknum" ::= "addr";
        "Data" ::= "blk"
      ] in
      struct.storeF Walog.S "memLog" "l" (SliceAppend (struct.loadF Walog.S "memLog" "l") "b");;
      Continue).

Definition MkLog: val :=
  λ: <>,
    let: "ml" := lock.new #() in
    let: "l" := struct.new Walog.S [
      "memLock" ::= "ml";
      "condLogger" ::= lock.newCond "ml";
      "condInstall" ::= lock.newCond "ml";
      "memLog" ::= NewSlice BlockData.T #0;
      "memStart" ::= #0;
      "diskEnd" ::= #0;
      "shutdown" ::= #false
    ] in
    Walog__recover "l";;
    Fork (Walog__logger "l");;
    Fork (Walog__installer "l");;
    "l".

Definition Walog__memWrite: val :=
  λ: "l" "bufs",
    struct.storeF Walog.S "memLog" "l" (SliceAppendSlice (struct.loadF Walog.S "memLog" "l") "bufs").

(* Assumes caller holds memLock
   XXX absorp *)
Definition Walog__doMemAppend: val :=
  λ: "l" "bufs",
    Walog__memWrite "l" "bufs";;
    let: "txn" := struct.loadF Walog.S "memStart" "l" + slice.len (struct.loadF Walog.S "memLog" "l") in
    "txn".

(* Scan log for blkno. If not present, read from disk
   XXX use map *)
Definition Walog__Read: val :=
  λ: "l" "blkno",
    let: "blk" := ref (zero_val (slice.T byteT)) in
    lock.acquire (struct.loadF Walog.S "memLock" "l");;
    (if: slice.len (struct.loadF Walog.S "memLog" "l") > #0
    then
      let: "i" := ref (slice.len (struct.loadF Walog.S "memLog" "l") - #1) in
      (for: (#true); ("i" <- !"i" - #1) :=
        let: "buf" := SliceGet (struct.loadF Walog.S "memLog" "l") !"i" in
        (if: BlockData.get "Blocknum" "buf" = "blkno"
        then
          "blk" <- NewSlice byteT disk.BlockSize;;
          xxcopy !"blk" (BlockData.get "Data" "buf");;
          Break
        else
          (if: !"i" = #0
          then Break
          else #()));;
        Continue);;
      #()
    else #());;
    lock.release (struct.loadF Walog.S "memLock" "l");;
    (if: !"blk" = slice.nil
    then
      "blk" <- disk.Read "blkno";;
      #()
    else #());;
    !"blk".

(* Append to in-memory log. Returns false, if bufs don't fit.
   Otherwise, returns the txn for this append. *)
Definition Walog__MemAppend: val :=
  λ: "l" "bufs",
    (if: slice.len "bufs" > Walog__LogSz "l"
    then (#0, #false)
    else
      let: "txn" := ref #0 in
      Skip;;
      (for: (#true); (Skip) :=
        lock.acquire (struct.loadF Walog.S "memLock" "l");;
        (if: struct.loadF Walog.S "memStart" "l" + slice.len (struct.loadF Walog.S "memLog" "l") - struct.loadF Walog.S "diskEnd" "l" + slice.len "bufs" > Walog__LogSz "l"
        then
          lock.release (struct.loadF Walog.S "memLock" "l");;
          Continue
        else
          "txn" <- Walog__doMemAppend "l" "bufs";;
          lock.release (struct.loadF Walog.S "memLock" "l");;
          Break));;
      (!"txn", #true)).

(* Wait until logger has appended in-memory log through txn to on-disk
   log *)
Definition Walog__LogAppendWait: val :=
  λ: "l" "txn",
    lock.acquire (struct.loadF Walog.S "memLock" "l");;
    Skip;;
    (for: (#true); (Skip) :=
      (if: "txn" ≤ struct.loadF Walog.S "diskEnd" "l"
      then Break
      else #());;
      Continue);;
    lock.release (struct.loadF Walog.S "memLock" "l").

(* Wait until last started transaction has been appended to log.  If
   it is logged, then all preceeding transactions are also logged. *)
Definition Walog__WaitFlushMemLog: val :=
  λ: "l",
    lock.acquire (struct.loadF Walog.S "memLock" "l");;
    let: "n" := struct.loadF Walog.S "memStart" "l" + slice.len (struct.loadF Walog.S "memLog" "l") in
    lock.release (struct.loadF Walog.S "memLock" "l");;
    Walog__LogAppendWait "l" "n".

(* Shutdown logger and installer *)
Definition Walog__Shutdown: val :=
  λ: "l",
    lock.acquire (struct.loadF Walog.S "memLock" "l");;
    struct.storeF Walog.S "shutdown" "l" #true;;
    lock.release (struct.loadF Walog.S "memLock" "l").
