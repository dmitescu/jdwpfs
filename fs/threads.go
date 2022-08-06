package fs

import (
	"context"
	"fmt"
	"strconv"
	"syscall"
	"log"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	jdwp "github.com/omerye/gojdb/jdwp"
)

//
// Errors
//
type JdwpThreadError struct {
	err error
	message string
}

func (e JdwpThreadError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("jdwp thread error: %s", e.err)
	}

	return fmt.Sprintf("jdwp thread error: %s", e.message)
}

//
// Jdwp thread master directory
//
type JdwpThreadMasterDir struct {
	fs.Inode

	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*JdwpThreadMasterDir)(nil))
var _ = (fs.NodeReaddirer)((*JdwpThreadMasterDir)(nil))
var _ = (fs.NodeLookuper)((*JdwpThreadMasterDir)(nil))

func NewJdwpThreadMasterDir(ctx context.Context, conn *jdwp.Connection) (*JdwpThreadMasterDir, error) {
	newThreadDir := &JdwpThreadMasterDir {
		JdwpContext: ctx,
		JdwpConnection: conn,
	}

	return newThreadDir, nil
}

func (d *JdwpThreadMasterDir) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *JdwpThreadMasterDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	threadIds, err := d.JdwpConnection.GetAllThreads()
	if err != nil {
		log.Println("unable to read threads from the JVM")
		return nil, syscall.EADDRNOTAVAIL
	}

	var threadDirEntries []fuse.DirEntry
	for _, threadId := range threadIds {
		newThreadDir, err := NewJdwpThreadDir(d.JdwpContext, d.JdwpConnection, threadId)
		if err != nil {
			log.Printf("error creating thread dir: %s", err)
			return nil, syscall.EADDRNOTAVAIL
		}
		threadDirEntries =
			append(threadDirEntries, newThreadDir.GetDirEntry(ctx))
	}
	
	return fs.NewListDirStream(threadDirEntries), 0
}

func (d *JdwpThreadMasterDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	threadId, err := strconv.ParseUint(name, 10, 64)
	if err != nil {
		return nil, syscall.ENOENT
	}

	threadEntry, err := NewJdwpThreadDir(d.JdwpContext, d.JdwpConnection, jdwp.ThreadID(threadId))
	if err != nil {
		log.Printf("could not access thread with id %d\n", threadId)
		return nil, syscall.ENOENT
	}
	
	threadEntryInode := d.NewInode(
		ctx,
		threadEntry,
		fs.StableAttr{
			Mode: fuse.S_IFDIR,
		},
	)
	
	return threadEntryInode, syscall.F_OK
}

//
// Jdwp thread dir
//
type JdwpThreadDir struct {
	fs.Inode

	ThreadId jdwp.ThreadID
	
	JdwpContext context.Context
	JdwpConnection *jdwp.Connection	
}

var _ = (fs.NodeGetattrer)((*JdwpThreadDir)(nil))
// var _ = (fs.NodeOnAdder)((*JdwpThreadDir)(nil))
var _ = (fs.NodeReaddirer)((*JdwpThreadDir)(nil))
var _ = (fs.NodeLookuper)((*JdwpThreadDir)(nil))

func NewJdwpThreadDir(ctx context.Context, conn *jdwp.Connection, id jdwp.ThreadID) (*JdwpThreadDir, error) {
	newThreadDir := &JdwpThreadDir {
		ThreadId: id,
		JdwpContext: ctx,
		JdwpConnection: conn,
	}

	return newThreadDir, nil
}

func (d *JdwpThreadDir) GetDirEntry(ctx context.Context) fuse.DirEntry {
	return fuse.DirEntry {
		Mode: fuse.S_IFREG,
		Name: strconv.FormatUint(uint64(d.ThreadId), 10),
	}
}

func (d *JdwpThreadDir) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *JdwpThreadDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	threadDirContents := [...]string{"name", "threadStatus", "suspendStatus", "control"}
	var infoFiles []fuse.DirEntry
	for _, infoFileName := range threadDirContents {
		infoFileEntry := fuse.DirEntry {
			Mode: fuse.S_IFREG,
			Name: infoFileName,
		}
		infoFiles = append(infoFiles, infoFileEntry)
	}
	
	return fs.NewListDirStream(infoFiles), 0
}

func (d *JdwpThreadDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	switch name {
	case "name":
		threadName, err := d.JdwpConnection.GetThreadName(d.ThreadId)
		if err != nil {
			log.Printf("error getting thread name: %s", err)
			return nil, syscall.EBADF
		}
		nameFile := d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(threadName),
					Attr: fuse.Attr{
						Mode: 0444,
					},
				},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
		return nameFile, 0
	case "threadStatus":
		threadStatus, _, err := d.JdwpConnection.GetThreadStatus(d.ThreadId)
		if err != nil {
			log.Printf("error getting thread status: %s", err)
			return nil, syscall.EBADF
		}
		
		threadStatusFile := d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(threadStatus.String()),
					Attr: fuse.Attr{
						Mode: 0444,
					},
				},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
		return threadStatusFile, 0
	case "suspendStatus":
		_, suspendStatus, err := d.JdwpConnection.GetThreadStatus(d.ThreadId)
		if err != nil {
			log.Printf("error getting thread status: %s", err)
			return nil, syscall.EBADF
		}

		suspendStatusFile := d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(suspendStatus.String()),
					Attr: fuse.Attr{
						Mode: 0444,
					},
				},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
		return suspendStatusFile, 0
	case "control":
		controlFile := NewThreadControlFile(d.JdwpContext, d.JdwpConnection, d.ThreadId)
		controlFileInode := d.NewInode(
			ctx,
			&controlFile,
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
		return controlFileInode, 0
	default:
		return nil, syscall.ENOENT
	}
}

//
// Thread control file
//
type ThreadControlFile struct {
	fs.Inode

	ThreadId jdwp.ThreadID
	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*ThreadControlFile)(nil))
var _ = (fs.NodeOpener)((*ThreadControlFile)(nil))
var _ = (fs.NodeReader)((*ThreadControlFile)(nil))
var _ = (fs.NodeWriter)((*ThreadControlFile)(nil))

func NewThreadControlFile(ctx context.Context, conn *jdwp.Connection, id jdwp.ThreadID) ThreadControlFile {
	return ThreadControlFile {
		ThreadId: id,
		JdwpContext: ctx,
		JdwpConnection: conn,
	}
}

func (c *ThreadControlFile) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if flags & (
		syscall.O_APPEND |
		syscall.O_CLOEXEC |
		syscall.O_EXCL |
		syscall.O_NOCTTY) != 0 {
		return nil, 0, syscall.EBADR
	}

	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (d *ThreadControlFile) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0666
	out.Uid = 1000
	return 0
}

func (c *ThreadControlFile) Read(ctx context.Context, _ fs.FileHandle, dest []byte, offset int64) (fuse.ReadResult, syscall.Errno) {
	_, suspendStatus, err := c.JdwpConnection.GetThreadStatus(c.ThreadId)
	if err != nil {
		return nil, syscall.EACCES
	}

	var controlFileContents string
        switch int(suspendStatus) {
	case 0:
		controlFileContents = "running"
	case 1:
		controlFileContents = "suspended"
	default:
		controlFileContents = "not implemented"
	}

	if offset > int64(len(controlFileContents)) {
		return nil, syscall.EBADR
	}
	
	output := []byte(controlFileContents[offset:])

	return fuse.ReadResultData(output), 0
}

// mostly doesn't work, truncation has to be implemented
func (c *ThreadControlFile) Write(ctx context.Context, _ fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	_, suspendStatus, err := c.JdwpConnection.GetThreadStatus(c.ThreadId)
	if err != nil {
		return 0, syscall.EACCES
	}

	var writtenState jdwp.SuspendStatus
	var changed = false
        switch string(data) {
	case "running":
	case "1":
	case "running\n":
	case "1\n":
		writtenState = 1
	case "suspend":
	case "0":
	case "suspend\n":
	case "0\n":
		writtenState = 0
	default:
		return 0, syscall.EFAULT
	}

	log.Printf("written state: %d\n", writtenState)
	if suspendStatus != writtenState {
		changed = true
		switch writtenState {
		case 0:
			c.JdwpConnection.Suspend(c.ThreadId)
		case 1:
			c.JdwpConnection.Resume(c.ThreadId)
		default:
			return 0, syscall.EFAULT
			
		}
	}

	if changed {
		return uint32(len(data)), 0
	} else {
		return 0, 0
	}
}
