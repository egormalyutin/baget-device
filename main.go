package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fs"
	"github.com/hanwen/go-fuse/fuse"
)

var (
	devicePath = flag.String("path", "", "path to create baget device")
)

func genBaget() []byte {
	result := []string{}

	lines := 4 + rand.Intn(4)

	for i := 0; i < lines; i++ {
		w := 5 + rand.Intn(10)
		ln := []string{}
		for j := 0; j < w; j++ {
			ln = append(ln, words[rand.Intn(len(words))])
		}
		result = append(result, strings.Join(ln, " "))
	}

	return []byte(strings.ToUpper(strings.Join(result, "\n@\n")) + "\n")
}

type bytesFileHandle struct {
	content []byte
}

var _ = (fs.FileReader)((*bytesFileHandle)(nil))

func (fh *bytesFileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	end := off + int64(len(dest))
	if end > int64(len(fh.content)) {
		end = int64(len(fh.content))
	}

	return fuse.ReadResultData(fh.content[off:end]), 0
}

type bagetFile struct {
	fs.Inode
}

var _ = (fs.NodeOpener)((*bagetFile)(nil))

func (f *bagetFile) Open(ctx context.Context, openFlags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	// disallow writes
	if fuseFlags&(syscall.O_RDWR|syscall.O_WRONLY) != 0 {
		return nil, 0, syscall.EROFS
	}

	fh = &bytesFileHandle{
		content: []byte(genBaget()),
	}

	// Return FOPEN_DIRECT_IO so content is not cached.
	return fh, fuse.FOPEN_DIRECT_IO, 0
}

func main() {
	flag.Parse()

	if *devicePath == "" {
		log.Fatal("Please, set -path flag to path on which you want to create baget device")
	}

	rand.Seed(time.Now().UnixNano())

	mntDir, err := ioutil.TempDir("/tmp", "baget-device-")
	if err != nil {
		log.Fatal(err)
	}

	root := &fs.Inode{}

	// Mount the file system
	server, err := fs.Mount(mntDir, root, &fs.Options{
		MountOptions: fuse.MountOptions{Debug: false, AllowOther: true},

		OnAdd: func(ctx context.Context) {
			ch := root.NewPersistentInode(
				ctx,
				&bagetFile{},
				fs.StableAttr{Mode: syscall.S_IFREG})
			root.AddChild("baget", ch, true)
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	dp, err := filepath.Abs(*devicePath)
	if err != nil {
		log.Fatal(err)
	}

	err = os.Symlink(filepath.Join(mntDir, "baget"), dp)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)

	go func() {
		select {
		case <-c:
			if err := os.Remove(dp); err != nil {
				log.Fatal(err)
			}
			os.Exit(1)
		}
	}()

	fmt.Printf("cat %s to see a new bugurt\n", dp)

	// Serve the file system, until unmounted by calling fusermount -u
	server.Wait()
}
