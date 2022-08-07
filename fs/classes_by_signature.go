// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package fs

import (
	"context"
	"path/filepath"
	"net/url"
	"syscall"
	"log"
	"strconv"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	jdwp "github.com/omerye/gojdb/jdwp"
)

//
// Jdwp class master directory
//
type JdwpClassNamedMasterDir struct {
	fs.Inode

	AbsoluteMountpoint string
	
	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*JdwpClassNamedMasterDir)(nil))
var _ = (fs.NodeReaddirer)((*JdwpClassNamedMasterDir)(nil))
var _ = (fs.NodeLookuper)((*JdwpClassNamedMasterDir)(nil))

func NewJdwpClassNamedMasterDir(ctx context.Context, conn *jdwp.Connection, absMountpoint string) (*JdwpClassNamedMasterDir, error) {
	newClassDir := &JdwpClassNamedMasterDir {
		AbsoluteMountpoint: absMountpoint,
		JdwpContext: ctx,
		JdwpConnection: conn,
	}

	return newClassDir, nil
}

func (d *JdwpClassNamedMasterDir) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *JdwpClassNamedMasterDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	// classes directories
	classInfos, err := d.JdwpConnection.GetAllClasses()
	if err != nil {
		log.Println("unable to retrieve all classes")
		return nil, syscall.EFAULT
	}

	var classInfoNamedEntries []fuse.DirEntry
	for _, classInfo := range classInfos {
		classSignature := classInfo.Signature
		
		classNamedEntry := fuse.DirEntry {
			Mode: fuse.S_IFLNK,
			Name: url.PathEscape(classSignature),
		}
		
		classInfoNamedEntries =
			append(classInfoNamedEntries, classNamedEntry)
	}
	
	return fs.NewListDirStream(classInfoNamedEntries), 0
}

func (d *JdwpClassNamedMasterDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	searchedClassSignature, err := url.PathUnescape(name)
	if err != nil {
		log.Printf("unable to unescape name %s\n", name)
		return nil, syscall.EFAULT
	}

	var foundClassId jdwp.ReferenceTypeID
	allClassInfos, err := d.JdwpConnection.GetAllClasses()
	if err != nil {
		log.Printf("unable to get all class infos: %s\n", err)
		return nil, syscall.EBADF
	}

	for _, classInfo := range allClassInfos {
		classSignature := classInfo.Signature

		if classSignature == searchedClassSignature {
			foundClassId = classInfo.TypeID
		}
	}

	if foundClassId == 0 {
		log.Printf("unable to find thread with name %s\n", searchedClassSignature)
		return nil, syscall.EFAULT
	}

	symlinkPath :=  filepath.Join(
		d.AbsoluteMountpoint,
		"classes",
		strconv.FormatUint(uint64(foundClassId), 10),
	)
	
	classEntryInode := d.NewInode(
		ctx,
		&fs.MemSymlink {
			Data: []byte(symlinkPath),
			Attr: fuse.Attr { Mode: 0444 },
		},
		fs.StableAttr{
			Mode: fuse.S_IFLNK,
		},
	)
	
	return classEntryInode, syscall.F_OK
}
