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
    |- classes -- A  -- methodA          classes & methods
    |          \...  |- methodB
    |                \...
    |
    |- breakpoints -- breakpoint 1       breakpoints
    |              \...
    |
    |- hooks  -- hook 1 -- script.go     JIT'ed Go hooks to run
    |                   |- breakpoint    symlink to the breakpoint
    |                   \. method        symlink to the method
    \...
    
```

# TODO list

- read only files should be static (no RW/link/unlink/delete/creation/etc...)
- one should not be able to create files outside the `hooks` dir
- the `control` file must be implemented
- add the `classes` subdir
- add the `breakpoints` subdir
- add the `hooks` subdir
- allow JIT'ing the hook scripts
- probably ACL should receive more attention
- ... are you ready for this one?... TESTS
