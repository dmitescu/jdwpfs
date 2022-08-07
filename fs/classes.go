// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package fs

import (
	"context"
	"fmt"
	"syscall"
	"log"
	"strconv"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	jdwp "github.com/omerye/gojdb/jdwp"
)

//
// Errors
//
type JdwpClassError struct {
	err error
	message string
}

func (e JdwpClassError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("jdwp class error: %s", e.err)
	}

	return fmt.Sprintf("jdwp class error: %s", e.message)
}

//
// Jdwp class master directory
//
type JdwpClassMasterDir struct {
	fs.Inode

	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*JdwpClassMasterDir)(nil))
var _ = (fs.NodeReaddirer)((*JdwpClassMasterDir)(nil))
var _ = (fs.NodeLookuper)((*JdwpClassMasterDir)(nil))

func NewJdwpClassMasterDir(ctx context.Context, conn *jdwp.Connection) (*JdwpClassMasterDir, error) {
	newClassDir := &JdwpClassMasterDir {
		JdwpContext: ctx,
		JdwpConnection: conn,
	}

	return newClassDir, nil
}


func (d *JdwpClassMasterDir) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *JdwpClassMasterDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	// classes directories
	classInfos, err := d.JdwpConnection.GetAllClasses()
	if err != nil {
		log.Println("unable to retrieve all classes")
		return nil, syscall.EFAULT
	}

	var classInfoEntries []fuse.DirEntry
	for _, classInfo := range classInfos {
		newClassDir, err := NewJdwpClassInfoDir(d.JdwpContext, d.JdwpConnection, classInfo.TypeID)
		if err != nil {
			log.Printf("error creating class dir for %d: %s", classInfo.TypeID, err)
			return nil, syscall.EFAULT
		}

		classInfoEntries = append(classInfoEntries, newClassDir.GetDirEntry(ctx))
	}
	
	return fs.NewListDirStream(classInfoEntries), 0
}

func (d *JdwpClassMasterDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {	
	classId, err := strconv.ParseUint(name, 10, 64)
	if err != nil {
		return nil, syscall.ENOENT
	}

	classEntry, err := NewJdwpClassInfoDir(d.JdwpContext, d.JdwpConnection, jdwp.ReferenceTypeID(classId))
	if err != nil {
		log.Printf("could not access class with id %d\n", classId)
		return nil, syscall.ENOENT
	}	
	
	classEntryInode := d.NewInode(
		ctx,
		classEntry,
		fs.StableAttr{
			Mode: fuse.S_IFDIR,
		},
	)
	
	return classEntryInode, syscall.F_OK
}
