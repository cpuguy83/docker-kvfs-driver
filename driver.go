package main

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/cpuguy83/kvfs/fs"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/hanwen/go-fuse/fuse"
)

type kvfsDriver struct {
	home    string
	addrs   []string
	store   string
	volumes map[string]*vol
	count   map[*vol]int
	sync.Mutex
}

type vol struct {
	*fs.FS
	mountPoint string
	name       string
	srv        *fuse.Server
}

func newDriver(store, home string, addrs []string) *kvfsDriver {
	return &kvfsDriver{
		home:    home,
		addrs:   addrs,
		store:   store,
		volumes: make(map[string]*vol),
		count:   make(map[*vol]int),
	}
}

func (d *kvfsDriver) Create(req volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()
	if v, exists := d.volumes[req.Name]; exists {
		return resp(v.mountPoint)
	}

	v, err := d.create(req.Name, req.Options)
	if err != nil {
		return resp(err)
	}

	d.volumes[v.name] = v
	d.count[v] = 0
	return resp(v.mountPoint)
}

func (d *kvfsDriver) Get(req volume.Request) volume.Response {
	var res volume.Response
	d.Lock()
	defer d.Unlock()
	v, exists := d.volumes[req.Name]
	if !exists {
		return resp(fmt.Errorf("no such volume"))
	}
	res.Volume = &volume.Volume{
		Name:       v.name,
		Mountpoint: v.mountPoint,
	}
	return res
}

func (d *kvfsDriver) List(req volume.Request) volume.Response {
	var res volume.Response
	d.Lock()
	defer d.Unlock()
	var ls = make([]*volume.Volume, len(d.volumes))
	for _, vol := range d.volumes {
		v := &volume.Volume{
			Name:       vol.name,
			Mountpoint: vol.mountPoint,
		}
		ls = append(ls, v)
	}
	res.Volumes = ls
	return res
}

func (d *kvfsDriver) Remove(req volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()

	v := d.volumes[req.Name]
	if err := v.srv.Unmount(); err != nil {
		return resp(err)
	}

	if err := os.RemoveAll(getMountpoint(d.home, req.Name)); err != nil {
		return resp(err)
	}

	delete(d.volumes, req.Name)
	return resp(v.mountPoint)
}

func (d *kvfsDriver) Path(req volume.Request) volume.Response {
	return resp(getMountpoint(d.home, req.Name))
}

func (d *kvfsDriver) Mount(req volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()
	v := d.volumes[req.Name]
	d.count[v]++
	return resp(getMountpoint(d.home, req.Name))
}
func (d *kvfsDriver) Unmount(req volume.Request) volume.Response {
	d.Lock()
	defer d.Unlock()
	v := d.volumes[req.Name]
	d.count[v]--
	return resp(getMountpoint(d.home, req.Name))
}

func (d *kvfsDriver) create(name string, opts map[string]string) (*vol, error) {
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

	srv, err := kv.NewServer(mp)
	if err != nil {
		return nil, err
	}
	go srv.Serve()

	return &vol{kv, getMountpoint(d.home, name), name, srv}, nil
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

func resp(r interface{}) volume.Response {
	switch t := r.(type) {
	case error:
		return volume.Response{Err: t.Error()}
	case string:
		return volume.Response{Mountpoint: t}
	default:
		return volume.Response{Err: "bad value writing response"}
	}
}
