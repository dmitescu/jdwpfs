# Description & Synopsis

`jdwpfs` is a FUSE filesystem which is aimed at debugging the JVM
through the JDWP protocol.

Usage:

```
jdwpfs -h $JDWP_HOST -p $JDWP_PORT /tmp/mountpoint
```

# Files

The `jdwpfs` should provide a VFS, with the following structure. As this
is WIP, it can change any time.

```
mnt -- host
    |- port
    |- threads -- 1                      threads of the JVM process 
    |          |- 2   -- control         file to control the suspend status
    |          |      |- name            thread name
    |          |      |- threadStatus    thread status
    |          |      \. suspendStatus   suspend status
    |          \...
    |
    |- threads_by_name -- main           symlinks to threads
    |                  \- ...
    |
    |- classes -- 1  -- fieldInfo        classes & methods
    |          \...  |- methodInfo
    |                |- fields -- 1 -- name
    |                |         |    |- signature
    |                |         |    \- modifiers
    |                |         |- 2
    |                |         \...
    |                |- methods -- 1 -- name
    |                |          |    |- signature
    |                |          |    \- modifiers
    |                |          |- 2
    |                |          \...
    |                \...
    |
    |- classes_by_signature -- A         symlinks to classes
    |                       \...
    |- events -- custom event 1 -- control          event control
              |                 |- kind             kind
              |                 |- suspendPolicy    suspend policy
              |                 |- location         location directory
              |                 \- hooks            hooks directory
              \...
    
```

# How-to

`jdwpfs` is nothing more than a FUSE interface to debugged Java VM.

Its purpose is to help script debugging processes (via stow, Makefiles, and Go
plugins).

At the base two files containing information about the connection can be found,
together with the functional directories.

## Classes

The classes dir contains the ClassIDs of the currently loaded classes. Inside,
a class info hierarchy resides:

- signature - the canonical name of the class
- methodInfo - a file containing a newline separated list of methods
- fieldInfo - the same, but for fields
- methods - a directory with the corresponding methods and their info
- fields - a directory with the corresponding fields and their info

## Classes by signature

It's easier to grep something semi-human-readable, and then resolve the link.

Listing the directory is quite slow for now.

## Threads

A `control` file can be found. Writing 1 or 0 decides if all threads should be resumed
or suspended.

Additionally, thread ids can be found, as directories with the following information:
- control - write 1 or 0 to suspend or resume a thread
- name
- suspendStatus
- threadStatus

## Threads by name

Symlinks to the actual thread directories

## Events

Creating a new event is done by calling `mkdir` in this directory.

Currently, the only sanely supported events are related to fields or methods.

- control - a control file; 1 or 0 register or deregister the event
- kind - this specifies the event kind; one should consult the JDWP documentation
         for an in-depth explanation; or `github.com/omerye/gojdb/jdwp/event_kind.go`
		 for the enum definition; for a string->kind conversion, either check that file
		 or the `map[string]jdwp.EventKind` declared in this project
- suspendPolicy - the suspend behaviour of the event; this is documented in the same place
                  as the event kinds ;)
- location - a directory; this is used to symlink to either a field or a method, which reside
             under a class directory; WARNING: relative symlinks should work relative to the
			 CWD of the `jdwpfs` process
- hooks - a directory; linking here is done against a real Go plugin; the entrypoint is
          a function: `func JdwpfsPluginEntrypoint(name string, event jdwp.Event) error`

# TODO list

- read only files should be static (no RW/link/unlink/delete/creation/etc...)
- probably ACL should receive more attention
- refactoring
- tests
- documentation ( + secret plan )
- a socket example
