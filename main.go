package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/calavera/dkvolume"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/libkv/store/boltdb"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/libkv/store/zookeeper"
)

const (
	dockerPluginSocketPath = "/var/run/docker/plugins"
)

func init() {
	consul.Register()
	boltdb.Register()
	etcd.Register()
	zookeeper.Register()
}

type stringsFlag []string

func (s *stringsFlag) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *stringsFlag) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func (s *stringsFlag) GetAll() []string {
	var out []string
	for _, i := range *s {
		out = append(out, i)
	}
	return out
}

var (
	flAddrs  stringsFlag
	flStore  = flag.String("store", "", "Set the KV store type to use")
	flHome   = flag.String("home", "/var/run/kvfs", "home dir for volume storage")
	flDebug  = flag.Bool("debug", false, "enable debug logging")
	flListen = flag.String("listen", dockerPluginSocketPath+"/kvfs.sock", "socket to listen for connections on")
)

func main() {
	flag.Var(&flAddrs, "addr", "List of address to KV store")
	flag.Parse()

	if len(flAddrs) == 0 {
		logrus.Fatal("need at least one addr to connect to kv store")
	}

	if *flDebug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if _, err := os.Stat(*flHome); err != nil {
		if !os.IsNotExist(err) {
			logrus.Fatal(err)
		}
		logrus.Debugf("created home dir at %s", *flHome)
		if err := os.MkdirAll(*flHome, 0700); err != nil {
			logrus.Fatal(err)
		}
	}

	kvfs := newDriver(*flStore, *flHome, flAddrs.GetAll())

	signal.Trap(func() {
		kvfs.cleanup()
	})

	h := dkvolume.NewHandler(kvfs)
	if err := h.ServeUnix("root", *flListen); err != nil {
		logrus.Fatal(err)
	}
}
