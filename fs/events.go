// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package fs

import (
	"context"
	"fmt"
	"log"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	jdwp "github.com/omerye/gojdb/jdwp"

	"disroot.org/kitzman/jdwpfs/debug"
)

//
// Jdwp event error
//
type JdwpEventDirError struct {
	err error
	message string
}

func (e JdwpEventDirError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("jdwp event dir error: %s", e.err)
	}

	return fmt.Sprintf("jdwp event dir error: %s", e.message)
}


//
// Jdwp event master directory
//
type JdwpEventsMasterDir struct {
	fs.Inode
	
	JdwpContext context.Context
	JdwpConnection *jdwp.Connection

	registered bool
	absoluteMountpoint string
	manager *debug.EventManager
}

var _ = (fs.NodeGetattrer)((*JdwpEventsMasterDir)(nil))
var _ = (fs.NodeMkdirer)((*JdwpEventsMasterDir)(nil))
var _ = (fs.NodeReaddirer)((*JdwpEventsMasterDir)(nil))
var _ = (fs.NodeLookuper)((*JdwpEventsMasterDir)(nil))

func NewJdwpEventsMasterDir(ctx context.Context, conn *jdwp.Connection, absMountpoint string) (*JdwpEventsMasterDir, error) {
	manager, err := debug.NewEventManager(ctx, conn)
	if err != nil {
		log.Printf("unable to create event master dir: %s\n", err)
		return nil, JdwpEventDirError { err: err }
	}
	
	eventsDir := &JdwpEventsMasterDir {
		JdwpContext: ctx,
		JdwpConnection: conn,

		registered: false,
		manager: manager,
		absoluteMountpoint: absMountpoint,
	}

	return eventsDir, nil
}

func (d *JdwpEventsMasterDir) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *JdwpEventsMasterDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	events, err := d.manager.GetAllEvents()
	if err != nil {
		log.Printf("unable to get event master dir: %s\n", err)
		return nil, syscall.EBADFD
	}

	var dirListing = []fuse.DirEntry{}
	for _, event := range events {
		if err != nil {
			log.Printf("unable to get event %s: %s\n", event.Name, err)
			return nil, syscall.EBADFD
		}

		eventDirEntry := fuse.DirEntry {
			Mode: fuse.S_IFREG,
			Name: event.Name,
		}

		dirListing = append(dirListing, eventDirEntry)
	}

	return fs.NewListDirStream(dirListing), syscall.F_OK
}

func (d *JdwpEventsMasterDir) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	_, err := d.manager.CreateEvent(name)
	if err != nil {
		log.Printf("unable to create event dir %s: %s", name, err)
		return nil, syscall.EADDRNOTAVAIL
	}

	eventDir, err := JdwpEventDirFromDebuggingEvent(name, d.absoluteMountpoint, d.manager)
	if err != nil {
		log.Printf("unable to validate the creation of event dir %s: %s", name, err)
		return nil, syscall.EADDRNOTAVAIL
	}
	
	eventDirInode := d.NewInode(
		ctx,
		eventDir,
		fs.StableAttr{
			Mode: fuse.S_IFDIR,
		},
	)
	
	return eventDirInode, 0	
}

func (d *JdwpEventsMasterDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	event, err := d.manager.GetEvent(name)
	if err != nil {
		return nil, syscall.ENOENT
	}

	eventDir, err := JdwpEventDirFromDebuggingEvent(event.Name, d.absoluteMountpoint, d.manager)
	if err != nil {
		log.Printf("error creating dir for %s", name)
		return nil, syscall.EADDRNOTAVAIL
	}
	
	eventDirInode := d.NewInode(
		ctx,
		eventDir,
		fs.StableAttr{
			Mode: fuse.S_IFDIR,
		},
	)

	return eventDirInode, syscall.F_OK
}
