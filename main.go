package main

import (
	// "context"
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jessevdk/go-flags"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fs"

	jdwpfs "disroot.org/kitzman/jdwpfs/fs"
)

type Options struct {
	DebuggedHost string `short:"h" long:"host" description:"host of debugged JVM process"`
	DebuggedPort int `short:"p" long:"port" description:"port of debugged JVM process"`
}

func main() {
	var opts Options
	args, err := flags.ParseArgs(&opts, os.Args)
	
	if err != nil {
		os.Exit(0)
	}

	if len(args) != 2 {
		log.Fatalf("mountpoint not supplied: %s\n", args)
	}

	mountpoint := args[1]

	_, err = os.Stat(mountpoint)
	if err != nil {
		panic(err)
	}

	absoluteMountpoint, _ := filepath.Abs(mountpoint)
	
	log.Printf("mounting at %s\n", mountpoint)
	log.Printf("debugging at %s:%d\n", opts.DebuggedHost, opts.DebuggedPort)

	fuseOptions := &fs.Options{
		MountOptions: fuse.MountOptions { // these should be tunable
			AllowOther: true,
			MaxBackground: 8,
			FsName: "jdwpfs",
			Name: "jdwpfs",
		},
		
		UID: uint32(os.Getuid()),
		GID: uint32(os.Getgid()),
	}
	jdwpContext := context.Background()
	rootFs, err := jdwpfs.NewJdwpRootfs(jdwpContext, absoluteMountpoint, opts.DebuggedHost, opts.DebuggedPort)

	if err != nil {
		panic(err)
	}
	
	server, err := fs.Mount(mountpoint, rootFs, fuseOptions)

	if err != nil {
		log.Fatalf("mount failed: %s\n", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("got %s, quitting\n", sig)
		server.Unmount()
	}();
	
	server.Wait()
}
