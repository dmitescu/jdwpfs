package fs

import (
	"context"
	"strconv"
	"syscall"
	"path/filepath"
	"log"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	jdwp "github.com/omerye/gojdb/jdwp"
)

//
// Jdwp thread master directory
//
type JdwpThreadNamedDir struct {
	fs.Inode

	AbsoluteMountpoint string
	
	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*JdwpThreadNamedDir)(nil))
var _ = (fs.NodeReaddirer)((*JdwpThreadNamedDir)(nil))
var _ = (fs.NodeLookuper)((*JdwpThreadNamedDir)(nil))

func NewJdwpThreadNamedDir(ctx context.Context, conn *jdwp.Connection, absMountpoint string) (*JdwpThreadNamedDir, error) {
	newThreadDir := &JdwpThreadNamedDir {
		AbsoluteMountpoint: absMountpoint,
		JdwpContext: ctx,
		JdwpConnection: conn,
	}

	return newThreadDir, nil
}

func (d *JdwpThreadNamedDir) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *JdwpThreadNamedDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	threadIds, err := d.JdwpConnection.GetAllThreads()
	if err != nil {
		log.Println("unable to read threads from the JVM")
		return nil, syscall.EADDRNOTAVAIL
	}

	var threadDirNamedEntries []fuse.DirEntry
	for _, threadId := range threadIds {
		threadName, err := d.JdwpConnection.GetThreadName(threadId)
		if err != nil {
			log.Printf("failed to get name of thread %d\n", threadId)
			return nil, syscall.EBADF
		}
		
		threadNamedEntry := fuse.DirEntry {
			Mode: fuse.S_IFLNK,
			Name: threadName,
		}
		
		threadDirNamedEntries =
			append(threadDirNamedEntries, threadNamedEntry)
	}
	
	return fs.NewListDirStream(threadDirNamedEntries), 0
}

func (d *JdwpThreadNamedDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	searchedThreadName := name

	var foundThreadId jdwp.ThreadID
	threadIds, err := d.JdwpConnection.GetAllThreads()
	if err != nil {
		log.Printf("unable to get all thread ids: %s\n", err)
		return nil, syscall.EBADF
	}

	for _, threadId := range threadIds {
		threadName, err := d.JdwpConnection.GetThreadName(threadId)
		if err != nil {
			log.Printf("unable to get all thread ids: %s\n", err)
			return nil, syscall.EBADF
		}

		if threadName == searchedThreadName {
			foundThreadId = threadId
		}
	}

	if foundThreadId == 0 {
		log.Printf("unable to find thread with name %s\n", searchedThreadName)
		return nil, syscall.EBADF
	}

	symlinkPath :=  filepath.Join(
		d.AbsoluteMountpoint,
		"threads",
		strconv.FormatUint(uint64(foundThreadId), 10),
	)
	
	threadEntryInode := d.NewInode(
		ctx,
		&fs.MemSymlink {
			Data: []byte(symlinkPath),
			Attr: fuse.Attr { Mode: 0444 },
		},
		fs.StableAttr{
			Mode: fuse.S_IFLNK,
		},
	)
	
	return threadEntryInode, syscall.F_OK
}
