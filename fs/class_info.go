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
// Jdwp class info directory
//
type JdwpClassInfoDir struct {
	fs.Inode

	TypeId jdwp.ReferenceTypeID

	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*JdwpClassInfoDir)(nil))
var _ = (fs.NodeReaddirer)((*JdwpClassInfoDir)(nil))
var _ = (fs.NodeLookuper)((*JdwpClassInfoDir)(nil))

func NewJdwpClassInfoDir(ctx context.Context, conn *jdwp.Connection, typeId jdwp.ReferenceTypeID) (*JdwpClassInfoDir, error) {
	classInfo := &JdwpClassInfoDir {
		TypeId: typeId,
		JdwpContext: ctx,
		JdwpConnection: conn,
	}

	return classInfo, nil
}

func (d *JdwpClassInfoDir) GetDirEntry(ctx context.Context) fuse.DirEntry {
	return fuse.DirEntry {
		Mode: fuse.S_IFDIR,
		Name: strconv.FormatUint(uint64(d.TypeId), 10),
	}
}

func (c *JdwpClassInfoDir) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *JdwpClassInfoDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	threadDirContents := [...]string{"method_info", "field_info"}
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

func (d *JdwpClassInfoDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	switch name {
	case "method_info":
		methods, err := d.JdwpConnection.GetMethods(d.TypeId)
		if err != nil {
			log.Printf("error getting class methods of id %d: %s", d.TypeId, err)
			return nil, syscall.EBADF
		}

		var method_info = ""
		for _, method := range methods {
			method_info = fmt.Sprintf("%s%s\t%s\n", method_info, method.Name, method.Signature)
		}
		
		methodInfoFile := d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(method_info),
				Attr: fuse.Attr{
					Mode: 0444,
				},
			},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
		return methodInfoFile, 0
	case "field_info":
		fields, err := d.JdwpConnection.GetFields(d.TypeId)
		if err != nil {
			log.Printf("error getting class fields of id %d: %s", d.TypeId, err)
			return nil, syscall.EBADF
		}

		var field_info = ""
		for _, field := range fields {
			field_info = fmt.Sprintf("%s%s\t%s\n", field_info, field.Name, field.Signature)
		}
		
		methodInfoFile := d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(field_info),
				Attr: fuse.Attr{
					Mode: 0444,
				},
			},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
		return methodInfoFile, 0		
	default:
		return nil, syscall.ENOENT
	}
}
