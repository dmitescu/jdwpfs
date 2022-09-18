// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package fs

import (
	"context"
	"log"
	// "os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"disroot.org/kitzman/jdwpfs/debug"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	jdwp "github.com/omerye/gojdb/jdwp"
)

var (
	eventKindReprMap = map[string]jdwp.EventKind {
		"SingleStep": jdwp.SingleStep,
		"Breakpoint": jdwp.Breakpoint,
		"FramePop": jdwp.FramePop,
		"Exception": jdwp.Exception,
		"UserDefined": jdwp.UserDefined,
		"ThreadStart": jdwp.ThreadStart,
		"ThreadDeath": jdwp.ThreadDeath,
		"ClassPrepare": jdwp.ClassPrepare,
		"ClassUnload": jdwp.ClassUnload,
		"ClassLoad": jdwp.ClassLoad,
		"FieldAccess": jdwp.FieldAccess,
		"FieldModification": jdwp.FieldModification,
		"ExceptionCatch": jdwp.ExceptionCatch,
		"MethodEntry": jdwp.MethodEntry,
		"MethodExit": jdwp.MethodExit,
		"VMStart": jdwp.VMStart,
		"VMDeath": jdwp.VMDeath,
	}

	suspendPolicyReprMap = map[string]jdwp.SuspendPolicy {
		"SuspendNone": jdwp.SuspendNone,
		"SuspendEventThread": jdwp.SuspendEventThread,
		"SuspendAll": jdwp.SuspendAll,
	}

	
)


//
// EventControlFile
//

type EventControlFile struct {
	fs.Inode

	event *debug.DebuggingEvent
}

var _ = (fs.NodeOpener)((*EventControlFile)(nil))
var _ = (fs.NodeGetattrer)((*EventControlFile)(nil))
var _ = (fs.NodeSetattrer)((*EventControlFile)(nil))
var _ = (fs.NodeReader)((*EventControlFile)(nil))
var _ = (fs.NodeWriter)((*EventControlFile)(nil))

func NewEventControlFile(event *debug.DebuggingEvent) EventControlFile {
	return EventControlFile {
		event: event,
	}
}


func (c *EventControlFile) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if flags & (
		syscall.O_APPEND |
		syscall.O_CLOEXEC |
		syscall.O_EXCL |
		syscall.O_NOCTTY) != 0 {
		return nil, 0, syscall.EBADR
	}

	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (c *EventControlFile) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0660
	return 0
}

func (c *EventControlFile) Setattr(ctx context.Context, _ fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, _ := in.GetSize(); sz != 0 {
		return syscall.EBADR
	}
	
	out.Attr.Mode = in.Mode
	out.Atime = in.Atime
	out.Atimensec = in.Atimensec
	// out.Size = in.Size

	return syscall.F_OK	
}

func (c *EventControlFile) Read(ctx context.Context, _ fs.FileHandle, dest []byte, offset int64) (fuse.ReadResult, syscall.Errno) {
	isRunning := c.event.IsRunning()
	var readString string
	switch isRunning {
	case true:
		readString = "running"
	case false:
		readString = "idle"
	}
	
	if offset > int64(len(readString)) {
		return nil, syscall.EBADR
	}

	return fuse.ReadResultData([]byte(readString[offset:])), syscall.F_OK
}

func (c *EventControlFile) Write(ctx context.Context, _ fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	writtenData := strings.TrimSpace(string(data))
	switch writtenData {
	case "run":
	case "1":
		if c.event.IsRunning() {
			return 0, syscall.ENAVAIL
		}

		_, err := c.event.Run()
		if err != nil {
			log.Printf("error running event %s: %s", c.event.Name, err)
			return 0, syscall.EBADE
		}
	case "cancel":
	case "0":
		if !c.event.IsRunning() {
			return 0, syscall.ENAVAIL
		}

		err := c.event.Cancel()
		if err != nil {
			log.Printf("error cancelling event %s: %s", c.event.Name, err)
			return 0, syscall.EBADE
		}
	default:
		return 0, syscall.EBADMSG
	}
	
	return uint32(len(data)), syscall.F_OK
}

//
// Event kind file
//

type EventKindFile struct {
	fs.Inode
	event *debug.DebuggingEvent
}

var _ = (fs.NodeOpener)((*EventKindFile)(nil))
var _ = (fs.NodeGetattrer)((*EventKindFile)(nil))
var _ = (fs.NodeSetattrer)((*EventKindFile)(nil))
var _ = (fs.NodeReader)((*EventKindFile)(nil))
var _ = (fs.NodeWriter)((*EventKindFile)(nil))

func NewEventKindFile(event *debug.DebuggingEvent) EventKindFile {
	return EventKindFile {
		event: event,
	}
}


func (c *EventKindFile) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if flags & (
		syscall.O_APPEND |
		syscall.O_CLOEXEC |
		syscall.O_EXCL |
		syscall.O_NOCTTY) != 0 {
		return nil, 0, syscall.EBADR
	}

	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (c *EventKindFile) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0660
	return 0
}

func (c *EventKindFile) Setattr(ctx context.Context, _ fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, _ := in.GetSize(); sz != 0 {
		return syscall.EBADR
	}
	
	out.Attr.Mode = in.Mode
	out.Atime = in.Atime
	out.Atimensec = in.Atimensec
	// out.Size = in.Size

	return syscall.F_OK	
}

func (c *EventKindFile) Read(ctx context.Context, _ fs.FileHandle, dest []byte, offset int64) (fuse.ReadResult, syscall.Errno) {
	kind := c.event.GetKind()
	readString := kind.String()

	if offset > int64(len(readString)) {
		return nil, syscall.EBADR
	}

	return fuse.ReadResultData([]byte(readString[offset:])), syscall.F_OK
}

func (c *EventKindFile) Write(ctx context.Context, _ fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	writtenData := strings.TrimSpace(string(data))
	eventKind, ok := eventKindReprMap[writtenData]
	if !ok {
		return 0, syscall.EAFNOSUPPORT
	}

	c.event.SetKind(eventKind)

	return uint32(len(data)), syscall.F_OK
}

//
// Event suspend policy file
//
type EventSuspendPolicyFile struct {
	fs.Inode
	event *debug.DebuggingEvent
}

var _ = (fs.NodeOpener)((*EventSuspendPolicyFile)(nil))
var _ = (fs.NodeGetattrer)((*EventSuspendPolicyFile)(nil))
var _ = (fs.NodeSetattrer)((*EventSuspendPolicyFile)(nil))
var _ = (fs.NodeReader)((*EventSuspendPolicyFile)(nil))
var _ = (fs.NodeWriter)((*EventSuspendPolicyFile)(nil))

func NewEventSuspendPolicyFile(event *debug.DebuggingEvent) EventSuspendPolicyFile {
	return EventSuspendPolicyFile {
		event: event,
	}
}


func (c *EventSuspendPolicyFile) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if flags & (
		syscall.O_APPEND |
		syscall.O_CLOEXEC |
		syscall.O_EXCL |
		syscall.O_NOCTTY) != 0 {
		return nil, 0, syscall.EBADR
	}

	return nil, fuse.FOPEN_DIRECT_IO, 0
}

func (c *EventSuspendPolicyFile) Getattr(ctx context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0660
	return 0
}

func (c *EventSuspendPolicyFile) Setattr(ctx context.Context, _ fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if sz, _ := in.GetSize(); sz != 0 {
		return syscall.EBADR
	}
	
	out.Attr.Mode = in.Mode
	out.Atime = in.Atime
	out.Atimensec = in.Atimensec
	// out.Size = in.Size

	return syscall.F_OK	
}

func (c *EventSuspendPolicyFile) Read(ctx context.Context, _ fs.FileHandle, dest []byte, offset int64) (fuse.ReadResult, syscall.Errno) {
	suspendPolicy := c.event.GetSuspendPolicy()
	readString := suspendPolicy.String()

	if offset > int64(len(readString)) {
		return nil, syscall.EBADR
	}
	
	return fuse.ReadResultData([]byte(readString[offset:])), syscall.F_OK
}

func (c *EventSuspendPolicyFile) Write(ctx context.Context, _ fs.FileHandle, data []byte, off int64) (written uint32, errno syscall.Errno) {
	writtenData := strings.TrimSpace(string(data))
	suspendPolicy, ok := suspendPolicyReprMap[writtenData]
	if !ok {
		log.Printf("unsupported suspend policy: %s\n", writtenData)
		return 0, syscall.EAFNOSUPPORT
	}

	c.event.SetSuspendPolicy(suspendPolicy)

	return uint32(len(data)), syscall.F_OK
}


//
// Event location directory
//

type EventLocationDirectory struct {
	fs.Inode

	JdwpConnection *jdwp.Connection
	event *debug.DebuggingEvent
	absoluteMountpoint string

	mu sync.RWMutex
	links [](struct {
		name string
		target string
	})
}

var _ = (fs.NodeGetattrer)((*EventLocationDirectory)(nil))
var _ = (fs.NodeSymlinker)((*EventLocationDirectory)(nil))
var _ = (fs.NodeUnlinker)((*EventLocationDirectory)(nil))
var _ = (fs.NodeReaddirer)((*EventLocationDirectory)(nil))
var _ = (fs.NodeLookuper)((*EventLocationDirectory)(nil))

func NewEventLocationDirectory(event *debug.DebuggingEvent, conn *jdwp.Connection, absMountpoint string) EventLocationDirectory {
	return EventLocationDirectory {
		event: event,
		JdwpConnection: conn,
		mu: sync.RWMutex{},
		absoluteMountpoint: absMountpoint,	
		links: [](struct {
			name string
			target string
		}){},
	}
}

func (d *EventLocationDirectory) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *EventLocationDirectory) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var entries = []fuse.DirEntry{}
	for _, link := range d.links {
		newEntry := fuse.DirEntry {
			Mode: fuse.S_IFLNK,
			Name: link.name,
		}
		entries = append(entries, newEntry)
	}
	
	return fs.NewListDirStream(entries), syscall.F_OK
}

func (d *EventLocationDirectory) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	absPathUneval, err := filepath.Abs(target)
	if err != nil {
		log.Printf("target %s cannot be made absolute: %s\n", target, err)
		return nil, syscall.ENOENT
	}
	
	absPath, err := filepath.EvalSymlinks(absPathUneval)
	if err != nil {
		log.Printf("target %s cannot be evaluated: %s\n", target, err)
		return nil, syscall.ENOENT
	}
	
	if !strings.HasPrefix(absPath, d.absoluteMountpoint) {
		log.Printf("target %s is not part of the current mount\n", target)
		return nil, syscall.EBADE
	}
	
	pathComponents := strings.Split(strings.TrimPrefix(absPath, d.absoluteMountpoint), "/")
	for pathComponents[0] == "/" || pathComponents[0] == "" {
		pathComponents = pathComponents[1:]
	}

	// classes/classid/methods/fields/method/field
	if !(len(pathComponents) == 4 &&
		 pathComponents[0] == "classes" &&
		 (pathComponents[2] == "fields" || pathComponents[2] == "methods")) {
		log.Printf("target %s does not seem to be correct\n", target)
		return nil, syscall.EBADE
	}

	var newModifier jdwp.EventModifier

	classId, err := strconv.ParseUint(pathComponents[1], 10, 64)
	if err != nil {
		log.Printf("target %s has unparsable class id\n", target)
		return nil, syscall.EBADE
	}

	var foundClass *jdwp.ClassInfo = nil
	classes, err := d.JdwpConnection.GetAllClasses()
	if err != nil {
		log.Printf("unable to retrieve classes for target %s\n", target)
		return nil, syscall.EADDRNOTAVAIL
	}
	
	for _, class := range classes {
		if class.ClassID() == jdwp.ClassID(classId) {
			foundClass = &class
		}
	}
	if foundClass == nil {
		log.Printf("unable to find a valid class for target %s\n", target)
		return nil, syscall.ENOENT
	}
	
	switch (pathComponents[2]) {
	case "fields":
		fieldId, err := strconv.ParseUint(pathComponents[3], 10, 64)
		if err != nil {
			log.Printf("target %s has unparsable field id\n", target)
			return nil, syscall.EBADE
		}


		var foundField *jdwp.Field = nil
		fields, err := d.JdwpConnection.GetFields(jdwp.ReferenceTypeID(classId))
		if err != nil {
			log.Printf("unable to retrieve fields for target %s\n", target)
			return nil, syscall.EADDRNOTAVAIL
		}
		
		for _, field := range fields {
			if field.ID == jdwp.FieldID(fieldId) {
				foundField = &field
			}
		}
		if foundField == nil {
			log.Printf("unable to find valid field for target %s\n", target)
			return nil, syscall.ENOENT
		}

		newModifier = jdwp.FieldOnlyEventModifier {
			Type: foundClass.TypeID,
			Field: foundField.ID,
		}
	case "methods":
		methodId, err := strconv.ParseUint(pathComponents[3], 10, 64)
		if err != nil {
			log.Printf("target %s has unparsable method id\n", target)
			return nil, syscall.EBADE
		}

		var foundMethod *jdwp.Method = nil

		methods, err := d.JdwpConnection.GetMethods(jdwp.ReferenceTypeID(classId))
		if err != nil {
			log.Printf("unable to retrieve methods for target %s\n", target)
			return nil, syscall.EADDRNOTAVAIL
		}

		for _, method := range methods {
			if method.ID == jdwp.MethodID(methodId) {
				foundMethod = &method
			}
		}
		if foundMethod == nil {
			log.Printf("unable to find matching method for target %s\n", target)
			return nil, syscall.ENOENT
		}

		newModifier = jdwp.LocationOnlyEventModifier(jdwp.Location {
			Type: foundClass.Kind,
			Class: foundClass.ClassID(),
			Method: foundMethod.ID,
			Location: 0,
		})
	default:
		log.Printf("target %s is not available", target)
		return nil, syscall.EADDRNOTAVAIL
	}
	d.event.SetModifier(name, newModifier)
	
	d.mu.Lock()
	defer d.mu.Unlock()
	
	newLink := d.NewInode(
		ctx,
		&fs.MemSymlink {
			Data: []byte(target),
			Attr: fuse.Attr { Mode: 0444 },
		},
		fs.StableAttr {
			Mode: fuse.S_IFLNK,
	})

	d.links = append(d.links, struct {
		name string
		target string
	}{
		name: name,
		target: target,
	})
	
	return newLink, syscall.F_OK
}

func (d *EventLocationDirectory) Unlink(ctx context.Context, name string) syscall.Errno {
	d.mu.Lock()
	defer d.mu.Unlock()

	var foundLinkIndex int
	var foundLink *struct {
		name string
		target string
	}
	for i, link := range d.links {
		if link.name == name {
			foundLink = &link
			foundLinkIndex = i
		}
	}
	if foundLink == nil {
		return syscall.ENOENT
	}

	d.links = append(
		d.links[:foundLinkIndex],
		d.links[(foundLinkIndex + 1):]...)
	
	return syscall.F_OK
}

func (d *EventLocationDirectory) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var foundLink *struct {
		name string
		target string
	}
	for _, link := range d.links {
		if link.name == name {
			foundLink = &link
		}
	}

	if foundLink == nil {
		return nil, syscall.ENOENT
	}
	
	hookLink := d.NewInode(
		ctx,
		&fs.MemSymlink {
			Data: []byte(foundLink.target),
			Attr: fuse.Attr { Mode: 0444 },
		},
		fs.StableAttr{
			Mode: fuse.S_IFLNK,
		},
	)

	return hookLink, syscall.F_OK
}


//
// Event hooks directory
//

type EventHooksDirectory struct {
	fs.Inode
	event *debug.DebuggingEvent

	mu sync.RWMutex
	links []struct {
		name string
		target string
	}
}

var _ = (fs.NodeGetattrer)((*EventHooksDirectory)(nil))
var _ = (fs.NodeReaddirer)((*EventHooksDirectory)(nil))
var _ = (fs.NodeSymlinker)((*EventHooksDirectory)(nil))
var _ = (fs.NodeLookuper)((*EventHooksDirectory)(nil))

func NewEventHooksDirectory(event *debug.DebuggingEvent) EventHooksDirectory {
	return EventHooksDirectory {
		event: event,
		mu: sync.RWMutex{},
		links: []struct {
			name string
			target string
		}{},
	}
}

func (d *EventHooksDirectory) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Mode = 0755
	return 0
}

func (d *EventHooksDirectory) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fs.Inode, errno syscall.Errno) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	newLink := d.NewInode(
		ctx,
		&fs.MemSymlink {
			Data: []byte(target),
			Attr: fuse.Attr { Mode: 0444 },
		},
		fs.StableAttr {
			Mode: fuse.S_IFLNK,
	})

	d.links = append(d.links, struct {
		name string
		target string
	}{
		name: name,
		target: target,
	})
	
	return newLink, syscall.F_OK
}

func (d *EventHooksDirectory) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var entries = []fuse.DirEntry{}
	for _, link := range d.links {
		newEntry := fuse.DirEntry {
			Mode: fuse.S_IFLNK,
			Name: link.name,
		}
		entries = append(entries, newEntry)
	}
	
	return fs.NewListDirStream(entries), syscall.F_OK
}

func (d *EventHooksDirectory) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var foundLink *struct {
		name string
		target string
	}
	for _, link := range d.links {
		if link.name == name {
			foundLink = &link
		}
	}

	if foundLink == nil {
		return nil, syscall.ENOENT
	}
	
	hookLink := d.NewInode(
		ctx,
		&fs.MemSymlink {
			Data: []byte(foundLink.target),
			Attr: fuse.Attr { Mode: 0444 },
		},
		fs.StableAttr{
			Mode: fuse.S_IFLNK,
		},
	)

	return hookLink, syscall.F_OK
}
