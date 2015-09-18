package main

import (
	"os"
	"path"
	"strings"
	"sync"

	"github.com/calavera/dkvolume"
	"github.com/cpuguy83/kvfs/fs"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type kvfsDriver struct {
	home    string
	addrs   []string
	store   string
	volumes map[string]*volume
	count   map[*volume]int
	sync.Mutex
}

type volume struct {
	*pathfs.PathNodeFs
	mountPoint string
	srv        *fuse.Server
}

func newDriver(store, home string, addrs []string) *kvfsDriver {
	return &kvfsDriver{
		home:    home,
		addrs:   addrs,
		store:   store,
		volumes: make(map[string]*volume),
		count:   make(map[*volume]int),
	}
}

func (d *kvfsDriver) Create(req dkvolume.Request) dkvolume.Response {
	d.Lock()
	defer d.Unlock()
	if v, exists := d.volumes[req.Name]; exists {
		return resp(v.mountPoint)
	}

	v, err := d.create(req.Name, req.Options)
	if err != nil {
		return resp(err)
	}

	d.volumes[req.Name] = v
	d.count[v] = 0
	return resp(v.mountPoint)
}

func (d *kvfsDriver) Remove(req dkvolume.Request) dkvolume.Response {
	d.Lock()
	defer d.Unlock()

	v := d.volumes[req.Name]
	if err := v.srv.Unmount(); err != nil {
		resp(err)
	}

	if err := os.RemoveAll(getMountpoint(d.home, req.Name)); err != nil {
		return resp(err)
	}

	delete(d.volumes, req.Name)
	return resp(v.mountPoint)
}

func (d *kvfsDriver) Path(req dkvolume.Request) dkvolume.Response {
	return resp(getMountpoint(d.home, req.Name))
}

func (d *kvfsDriver) Mount(req dkvolume.Request) dkvolume.Response {
	d.Lock()
	defer d.Unlock()
	v := d.volumes[req.Name]
	d.count[v]++
	return resp(getMountpoint(d.home, req.Name))
}
func (d *kvfsDriver) Unmount(req dkvolume.Request) dkvolume.Response {
	d.Lock()
	defer d.Unlock()
	v := d.volumes[req.Name]
	d.count[v]--
	return resp(getMountpoint(d.home, req.Name))
}

func (d *kvfsDriver) create(name string, opts map[string]string) (*volume, error) {
	store := d.store
	if s, exists := opts["store"]; exists {
		store = s
	}

	addrs := d.addrs
	if a, exists := opts["addrs"]; exists {
		addrs = getAddrs(a)
	}
	kvOpts := fs.Options{
		Store: store,
		Addrs: addrs,
		Root:  opts["root"],
	}
	kv, err := fs.NewKVFS(kvOpts)
	if err != nil {
		return nil, err
	}

	mp := getMountpoint(d.home, name)
	if err := os.MkdirAll(mp, 0700); err != nil {
		return nil, err
	}
	srv, _, err := nodefs.MountRoot(mp, kv.Root(), nil)
	if err != nil {
		return nil, err
	}
	go srv.Serve()

	return &volume{kv, getMountpoint(d.home, name), srv}, nil
}

func (d *kvfsDriver) cleanup() {
	for _, v := range d.volumes {
		if err := v.srv.Unmount(); err == nil {
			os.RemoveAll(v.mountPoint)
		}
	}
	os.RemoveAll(d.home)
}

func getMountpoint(home, name string) string {
	return path.Join(home, name)
}

func getAddrs(addrs string) []string {
	return strings.Split(addrs, ",")
}

func resp(r interface{}) dkvolume.Response {
	switch t := r.(type) {
	case error:
		return dkvolume.Response{Err: t.Error()}
	case string:
		return dkvolume.Response{Mountpoint: t}
	default:
		return dkvolume.Response{Err: "bad value writing response"}
	}
}
