// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package fs

import (
	"context"
	"fmt"
	"syscall"
	"log"
	"strconv"
	"sort"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	jdwp "github.com/omerye/gojdb/jdwp"
)

//
// Method sorting helper
//

type MethodById []jdwp.Method
func (ms MethodById) Len() int { return len(ms) }
func (ms MethodById) Swap(i, j int) { ms[i], ms[j] = ms[j], ms[i] }
func (ms MethodById) Less(i, j int) bool { return ms[i].ID < ms[j].ID }

//
// Field sorting helper
//

type FieldById []jdwp.Field
func (fs FieldById) Len() int { return len(fs) }
func (fs FieldById) Swap(i, j int) { fs[i], fs[j] = fs[j], fs[i] }
func (fs FieldById) Less(i, j int) bool { return fs[i].ID < fs[j].ID }

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
	classDirContents := [...]string{"methodInfo", "fieldInfo", "methods", "fields"}
	var infoFiles []fuse.DirEntry
	for _, infoFileName := range classDirContents {
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
	case "methodInfo":
		methods, err := d.JdwpConnection.GetMethods(d.TypeId)
		if err != nil {
			log.Printf("error getting class methods of id %d: %s", d.TypeId, err)
			return nil, syscall.EBADF
		}
		
		sort.Sort(MethodById(methods))
		
		var method_info = ""
		for _, method := range methods {
			method_info = fmt.Sprintf("%s%d\t%s\t%s\n", method_info, uint64(method.ID), method.Name, method.Signature)
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
	case "fieldInfo":
		fields, err := d.JdwpConnection.GetFields(d.TypeId)
		if err != nil {
			log.Printf("error getting class fields of id %d: %s", d.TypeId, err)
			return nil, syscall.EBADF
		}

		sort.Sort(FieldById(fields))
		
		var field_info = ""
		for _, field := range fields {
			field_info = fmt.Sprintf("%s%d\t%s\t%s\n", field_info, uint64(field.ID), field.Name, field.Signature)
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
	case "methods":
		methodDir, err := NewClassMethodMasterDir(d.JdwpContext, d.JdwpConnection, d.TypeId)
		if err != nil {
			log.Printf("error creating method dir of class with id %d: %s", d.TypeId, err)
			return nil, syscall.EFAULT
		}

		methodDirFile := d.NewInode(
			ctx,
			methodDir,
			fs.StableAttr {
				Mode: fuse.S_IFDIR,
			},
		)
		return methodDirFile, fuse.F_OK
	case "fields":
		fieldDir, err := NewClassFieldMasterDir(d.JdwpContext, d.JdwpConnection, d.TypeId)
		if err != nil {
			log.Printf("error creating field dir of class with id %d: %s", d.TypeId, err)
			return nil, syscall.EFAULT
		}

		fieldDirFile := d.NewInode(
			ctx,
			fieldDir,
			fs.StableAttr {
				Mode: fuse.S_IFDIR,
			},
		)
		return fieldDirFile, fuse.F_OK
				
	default:
		return nil, syscall.ENOENT
	}
}

//
// Class method master directory
// Unfortunately, there is no way of having a name-based method directory, as methods
// can be overloaded. For finding out the method id, the `method_info` file can be used
//
type ClassMethodMasterDir struct {
	fs.Inode

	TypeId jdwp.ReferenceTypeID

	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*ClassMethodMasterDir)(nil))
var _ = (fs.NodeReaddirer)((*ClassMethodMasterDir)(nil))
var _ = (fs.NodeLookuper)((*ClassMethodMasterDir)(nil))

func NewClassMethodMasterDir(ctx context.Context, conn *jdwp.Connection, id jdwp.ReferenceTypeID) (*ClassMethodMasterDir, error) {
	masterDir := &ClassMethodMasterDir {
		TypeId: id,
		JdwpContext: ctx,
		JdwpConnection: conn,	
	}

	return masterDir, nil
}

func (d *ClassMethodMasterDir) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *ClassMethodMasterDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	methods, err := d.JdwpConnection.GetMethods(d.TypeId)
	
	if err != nil {
		log.Printf("unable to read methods for class id %d: %s\n", uint64(d.TypeId), err)
		return nil, syscall.EFAULT
	}

	var methodDirEntries []fuse.DirEntry
	for _, method := range methods {
		methodEntry := fuse.DirEntry {
			Mode: fuse.S_IFDIR,
			Name: strconv.FormatUint(uint64(method.ID), 10),
		}
		
		methodDirEntries =
			append(methodDirEntries, methodEntry)
	}
	
	return fs.NewListDirStream(methodDirEntries), 0
}

func (d *ClassMethodMasterDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	methodIdUint, err := strconv.ParseUint(name, 10, 64)
	if err != nil {
		log.Printf("unable to parse id %s\n", name)
		return nil, syscall.ENOENT
	}
	methodId := jdwp.MethodID(methodIdUint)
	
	methods, err := d.JdwpConnection.GetMethods(d.TypeId)
	if err != nil {
		log.Printf("unable to read methods for class id %d: %s\n", uint64(d.TypeId), err)
		return nil, syscall.EFAULT
	}

	var method jdwp.Method
	var methodFound bool = false
	for _, foundMethod := range methods {
		if foundMethod.ID == methodId {
			method = foundMethod
			methodFound = true
		}
	}

	if !methodFound {
		log.Printf("unable to find method %d in class %d\n", methodId, d.TypeId)
		return nil, syscall.ENOENT
	}

	methodFile, err := NewClassMethodDir(d.JdwpContext, d.JdwpConnection, d.TypeId, method.ID)
	if err != nil {
		log.Printf("unable to create dir for method with id %d\n", method.ID)
		return nil, syscall.EFAULT
	}

	methodFileInode := d.NewInode(
		ctx,
		methodFile,
		fs.StableAttr {
			Mode: fuse.S_IFDIR,
		},)

	return methodFileInode, syscall.F_OK
}

//
// Class field master directory
// Unfortunately, there is no way of having a name-based method directory, as methods
// can be overloaded. For finding out the method id, the `field_info` file can be used
//
type ClassFieldMasterDir struct {
	fs.Inode

	TypeId jdwp.ReferenceTypeID

	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*ClassFieldMasterDir)(nil))
var _ = (fs.NodeReaddirer)((*ClassFieldMasterDir)(nil))
var _ = (fs.NodeLookuper)((*ClassFieldMasterDir)(nil))

func NewClassFieldMasterDir(ctx context.Context, conn *jdwp.Connection, id jdwp.ReferenceTypeID) (*ClassFieldMasterDir, error) {
	masterDir := &ClassFieldMasterDir {
		TypeId: id,
		JdwpContext: ctx,
		JdwpConnection: conn,	
	}

	return masterDir, nil
}

func (d *ClassFieldMasterDir) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *ClassFieldMasterDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	fields, err := d.JdwpConnection.GetFields(d.TypeId)
	
	if err != nil {
		log.Printf("unable to read fields for class id %d: %s\n", uint64(d.TypeId), err)
		return nil, syscall.EFAULT
	}

	var fieldDirEntries []fuse.DirEntry
	for _, field := range fields {
		fieldEntry := fuse.DirEntry {
			Mode: fuse.S_IFDIR,
			Name: strconv.FormatUint(uint64(field.ID), 10),
		}
		
		fieldDirEntries =
			append(fieldDirEntries, fieldEntry)
	}
	
	return fs.NewListDirStream(fieldDirEntries), 0
}
func (d *ClassFieldMasterDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	fieldIdUint, err := strconv.ParseUint(name, 10, 64)
	if err != nil {
		log.Printf("unable to parse id %s\n", name)
		return nil, syscall.ENOENT
	}
	fieldId := jdwp.FieldID(fieldIdUint)
	
	fields, err := d.JdwpConnection.GetFields(d.TypeId)
	if err != nil {
		log.Printf("unable to read fields for class id %d: %s\n", uint64(d.TypeId), err)
		return nil, syscall.EFAULT
	}

	var field jdwp.Field
	var fieldFound bool = false
	for _, foundField := range fields {
		if foundField.ID == fieldId {
			field = foundField
			fieldFound = true
		}
	}

	if !fieldFound {
		log.Printf("unable to find field %d in class %d\n", fieldId, d.TypeId)
		return nil, syscall.ENOENT
	}

	fieldFile, err := NewClassFieldDir(d.JdwpContext, d.JdwpConnection, d.TypeId, field.ID)
	if err != nil {
		log.Printf("unable to create dir for field with id %d\n", field.ID)
		return nil, syscall.EFAULT
	}

	fieldFileInode := d.NewInode(
		ctx,
		fieldFile,
		fs.StableAttr {
			Mode: fuse.S_IFDIR,
		},)

	return fieldFileInode, syscall.F_OK
}

//
// Class method directory
//
type ClassMethodDir struct {
	fs.Inode

	TypeId jdwp.ReferenceTypeID
	MethodId jdwp.MethodID

	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*ClassMethodDir)(nil))
var _ = (fs.NodeReaddirer)((*ClassMethodDir)(nil))
var _ = (fs.NodeLookuper)((*ClassMethodDir)(nil))

func NewClassMethodDir(ctx context.Context, conn *jdwp.Connection, typeId jdwp.ReferenceTypeID, methodId jdwp.MethodID) (*ClassMethodDir, error) {
	methodDir := &ClassMethodDir {
		TypeId: typeId,
		MethodId: methodId,

		JdwpContext: ctx,
		JdwpConnection: conn,
	}

	return methodDir, nil
}

func (d *ClassMethodDir) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *ClassMethodDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	threadDirContents := [...]string{"name", "signature", "modifiers"}
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

func (d *ClassMethodDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	methods, err := d.JdwpConnection.GetMethods(d.TypeId)
	if err != nil {
		log.Printf("methods for class with id %d not found: %s", uint64(d.TypeId), err)
		return nil, syscall.EFAULT
	}

	var method jdwp.Method
	var methodFound bool = false
	for _, foundMethod := range methods {
		if foundMethod.ID == d.MethodId {
			method = foundMethod
			methodFound = true
		}
	}
	if !methodFound {
		log.Printf("unable to find the constructed method with id %d: %s\n", d.MethodId, err)
		return nil, syscall.EFAULT
	}

	var methodFile *fs.Inode

	switch name {
	case "name":
		methodFile = d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(method.Name),
				Attr: fuse.Attr{
					Mode: 0444,
				},
			},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
	case "signature":
		methodFile = d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(method.Signature),
				Attr: fuse.Attr{
					Mode: 0444,
				},
			},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
	case "modifiers":
		methodFile = d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(method.ModBits.String()),
				Attr: fuse.Attr{
					Mode: 0444,
				},
			},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
	default:
		return nil, syscall.ENOENT
	}
	return methodFile, 0
}

//
// Class field directory
//
type ClassFieldDir struct {
	fs.Inode

	TypeId jdwp.ReferenceTypeID
	FieldId jdwp.FieldID

	JdwpContext context.Context
	JdwpConnection *jdwp.Connection
}

var _ = (fs.NodeGetattrer)((*ClassFieldDir)(nil))
var _ = (fs.NodeReaddirer)((*ClassFieldDir)(nil))
var _ = (fs.NodeLookuper)((*ClassFieldDir)(nil))

func NewClassFieldDir(ctx context.Context, conn *jdwp.Connection, typeId jdwp.ReferenceTypeID, fieldId jdwp.FieldID) (*ClassFieldDir, error) {
	fieldDir := &ClassFieldDir {
		TypeId: typeId,
		FieldId: fieldId,

		JdwpContext: ctx,
		JdwpConnection: conn,
	}

	return fieldDir, nil
}

func (d *ClassFieldDir) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *ClassFieldDir) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	threadDirContents := [...]string{"name", "signature", "modifiers"}
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

func (d *ClassFieldDir) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	fields, err := d.JdwpConnection.GetFields(d.TypeId)
	if err != nil {
		log.Printf("fields for class with id %d not found: %s", uint64(d.TypeId), err)
		return nil, syscall.EFAULT
	}

	var field jdwp.Field
	var fieldFound bool = false
	for _, foundField := range fields {
		if foundField.ID == d.FieldId {
			field = foundField
			fieldFound = true
		}
	}
	if !fieldFound {
		log.Printf("unable to find the constructed field with id %d: %s\n", d.FieldId, err)
		return nil, syscall.EFAULT
	}

	var fieldFile *fs.Inode

	switch name {
	case "name":
		fieldFile = d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(field.Name),
				Attr: fuse.Attr{
					Mode: 0444,
				},
			},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
	case "signature":
		fieldFile = d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(field.Signature),
				Attr: fuse.Attr{
					Mode: 0444,
				},
			},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
	case "modifiers":
		fieldFile = d.NewInode(
			ctx,
			&fs.MemRegularFile {
				Data: []byte(field.ModBits.String()),
				Attr: fuse.Attr{
					Mode: 0444,
				},
			},
			fs.StableAttr {
				Mode: fuse.S_IFREG,
			})
	default:
		return nil, syscall.ENOENT
	}
	return fieldFile, 0
}
