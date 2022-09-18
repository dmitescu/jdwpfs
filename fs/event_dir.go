// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package fs

import (
	"context"
	"errors"
	"log"
	"syscall"
	
	"disroot.org/kitzman/jdwpfs/debug"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

//
// Jdwp event event directory
//
type JdwpEventDir struct {
	fs.Inode
	
	manager *debug.EventManager

	registered bool
	name string
	absoluteMountpoint string
	event *debug.DebuggingEvent
}

var _ = (fs.NodeGetattrer)((*JdwpEventDir)(nil))
var _ = (fs.NodeUnlinker)((*JdwpEventDir)(nil))
var _ = (fs.NodeReaddirer)((*JdwpEventDir)(nil))
var _ = (fs.NodeLookuper)((*JdwpEventDir)(nil))

// func NewJdwpEventDir(manager *debug.EventManager, name string) (*JdwpEventDir, error) {
// 	event := debug.NewStubDebuggingEvent(name)

// 	eventDir := &JdwpEventDir {
// 		JdwpContext: manager.JdwpContext,
// 		JdwpConnection: manager.JdwpConnection,
			
// 		manager: manager,

// 		name: name,
// 		registered: false,
// 		event: event,
// 	}

// 	return eventDir, nil
// }

func JdwpEventDirFromDebuggingEvent(name string, absMountpoint string, manager *debug.EventManager) (*JdwpEventDir, error) {
	event, err := manager.GetEvent(name)
	if errors.As(err, &debug.JdwpDebuggingEventError{}) {
		log.Printf("inaccessible dir")
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	
	eventDir := &JdwpEventDir {
		manager: manager,

		name: name,
		registered: true,
		absoluteMountpoint: absMountpoint,
		event: event,
	}

	return eventDir, nil
}

func (d *JdwpEventDir) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *JdwpEventDir) Unlink(_ context.Context, name string) syscall.Errno {
	if (name == "registered") {
		if (d.registered) {
			return syscall.EROFS
		}
		
		err := d.manager.DeregisterEvent(name)
		if err != nil {
			log.Printf("error deregistering event %s: %s", name, err)
			return syscall.ECANCELED
		}

		d.registered = true

		return syscall.F_OK
	}
	
	return syscall.EROFS;
}

func (d *JdwpEventDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	registeredEntry := fuse.DirEntry {
		Mode: fuse.S_IFREG,
		Name: "control",
	}

	kindEntry := fuse.DirEntry {
		Mode: fuse.S_IFREG,
		Name: "kind",
	}
	
	suspendPolicyEntry := fuse.DirEntry {
		Mode: fuse.S_IFREG,
		Name: "suspendPolicy",
	}

	locationEntry := fuse.DirEntry {
		Mode: fuse.S_IFDIR,
		Name: "location",
	}

	hooksEntry := fuse.DirEntry {
		Mode: fuse.S_IFDIR,
		Name: "hooks",
	}

	dirListing := []fuse.DirEntry {
		registeredEntry,
		kindEntry,
		suspendPolicyEntry,
		locationEntry,
		hooksEntry,
	}
	
	return fs.NewListDirStream(dirListing), syscall.F_OK
}

func (d *JdwpEventDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	switch (name) {
	case "control":
		foundFile := NewEventControlFile(d.event)
		foundInode := d.NewInode(
			ctx,
			&foundFile,
			fs.StableAttr{
				Mode: fuse.S_IFREG,
			},
		)
		return foundInode, syscall.F_OK
	case "kind":
		foundFile := NewEventKindFile(d.event)
		foundInode := d.NewInode(
			ctx,
			&foundFile,
			fs.StableAttr{
				Mode: fuse.S_IFREG,
			},
		)
		return foundInode, syscall.F_OK
	case "suspendPolicy":
		foundFile := NewEventSuspendPolicyFile(d.event)
		foundInode := d.NewInode(
			ctx,
			&foundFile,
			fs.StableAttr{
				Mode: fuse.S_IFREG,
			},
		)
		return foundInode, syscall.F_OK
	case "hooks":
		foundFile := NewEventHooksDirectory(d.event)
		foundInode := d.NewInode(
			ctx,
			&foundFile,
			fs.StableAttr{
				Mode: fuse.S_IFDIR,
			},
		)
		return foundInode, syscall.F_OK
	case "location":
		foundFile := NewEventLocationDirectory(d.event, d.manager.JdwpConnection, d.absoluteMountpoint)
		foundInode := d.NewInode(
			ctx,
			&foundFile,
			fs.StableAttr{
				Mode: fuse.S_IFDIR,
			},
		)
		return foundInode, syscall.F_OK
	default:
		return nil, syscall.ENOENT
	}
}
