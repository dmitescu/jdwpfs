// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package fs

import (
	"context"
	"fmt"
	"strconv"
	"syscall"
	"net"
	"log"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	jdwp "github.com/omerye/gojdb/jdwp"
)

//
// Errors
//
type JdwpProtocolError struct {
	err error
	message string
}

func (e JdwpProtocolError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("jdwp protocol error: %s", e.err)
	}

	return fmt.Sprintf("jdwp protocol error: %s", e.message)
}

//
// The JDWP filesystem root
//
type JdwpRootFs struct {
	fs.Inode
	
	AbsoluteMountpoint string
	
	Host string
	Port int

	Connection net.Conn

	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*JdwpRootFs)(nil))
var _ = (fs.NodeOnAdder)((*JdwpRootFs)(nil))

func NewJdwpRootfs(ctx context.Context, absMountpoint string, host string, port int) (*JdwpRootFs, error) {
	if port < 1 {
		return nil, JdwpProtocolError {
			message: fmt.Sprintf("port %d cannot exist", port),
		}
	}

	if host == "" {
		return nil, JdwpProtocolError {
			message: fmt.Sprintf("host '%s' cannot exist", host),
		}
	}

	tcpConnection, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, JdwpProtocolError { err: err }
	}

	jdwpConnection, err := jdwp.Open(ctx, tcpConnection)
	if err != nil {
		return nil, JdwpProtocolError { err: err }
	}

	newJdwpFs := &JdwpRootFs {
		AbsoluteMountpoint: absMountpoint,
		Host: host,
		Port: port,
		Connection: tcpConnection,
		JdwpContext: ctx,
		JdwpConnection: jdwpConnection,
	}
	
	return newJdwpFs, nil
}

func (r *JdwpRootFs) OnAdd(ctx context.Context) {
	// creation of informational files
	hostFile := r.NewPersistentInode(
		ctx, &fs.MemRegularFile{
			Data: []byte(r.Host),
			Attr: fuse.Attr{
				Mode: 0444,
			},
		}, fs.StableAttr{Ino: 2})
	
	portFile := r.NewPersistentInode(
		ctx, &fs.MemRegularFile{
			Data: []byte(strconv.Itoa(r.Port)),
			Attr: fuse.Attr{
				Mode: 0444,
			},
		}, fs.StableAttr{Ino: 3})

	// thread listing
	threadMasterDir, err := NewJdwpThreadMasterDir(r.JdwpContext, r.JdwpConnection)
	if err != nil {
		log.Panicf("could not create thread dir: %s", err)
	}
	threadMasterDirInode := r.NewPersistentInode(
		ctx,
		threadMasterDir,
		fs.StableAttr{
			Mode: fuse.S_IFDIR,
			Ino: 4,
		})

	// named thread listing
	threadNamedDir, err := NewJdwpThreadNamedDir(r.JdwpContext, r.JdwpConnection, r.AbsoluteMountpoint)
	if err != nil {
		log.Panicf("could not create named thread dir: %s", err)
	}
	threadNamedDirInode := r.NewPersistentInode(
		ctx,
		threadNamedDir,
		fs.StableAttr{
			Mode: fuse.S_IFDIR,
			Ino: 5,
		})

	// classes dir
	classesDir, err := NewJdwpClassMasterDir(r.JdwpContext, r.JdwpConnection)
	classesDirInode := r.NewPersistentInode(
		ctx,
		classesDir,
		fs.StableAttr{
			Mode: fuse.S_IFDIR,
			Ino: 6,
		})

	// named classes dir
	classesNamedDir, err := NewJdwpClassNamedMasterDir(r.JdwpContext, r.JdwpConnection, r.AbsoluteMountpoint)
	classesNamedDirInode := r.NewPersistentInode(
		ctx,
		classesNamedDir,
		fs.StableAttr{
			Mode: fuse.S_IFDIR,
			Ino: 7,
		})

	
	// hooking files
	r.AddChild("host", hostFile, false)
	r.AddChild("port", portFile, false)

	r.AddChild("threads", threadMasterDirInode, false)
	r.AddChild("threads_by_name", threadNamedDirInode, false)

	r.AddChild("classes", classesDirInode, false)
	r.AddChild("classes_by_signature", classesNamedDirInode, false)
}

func (r *JdwpRootFs) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}
