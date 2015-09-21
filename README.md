# docker-kvfs-driver
Docker Volume Driver for distributed k/v stores

Connects to any supported k/v store and provides a FUSE filesystem to the container as a volume.

Supported K/V stores (thanks to https://github.com/docker/libkv):
- etcd
- zookeeper
- consul
- boltdb

### Usage

Start up the kvfs daemon:
```bash
$ docker-kvfs-driver --store <store name> --addr <addr1> --addr <addr2> &
```

With docker 1.8:
```bash
$ docker run -v /data --volume-driver kvfs busybox
```

With docker 1.9:
```bash
$ docker volume create --name my_config --driver kvfs [--opt store=<store name>|--opt root=<root k/v node to mount>|--opt addrs=<addr1,addr2,...>]
my_config
$ docker run -v my_config:/data busybox
```

The options provided to the driver daemon are considered default values for when it creates a volume.
The `--opt` values provided to `docker volume create` override those options.

#### Opts Description:

**root** - *sets the root node to mount into the container. By default it will use the base node.*  
For instance, your k/v store has a strucute like so:
```
/
|__ foo
      |__ a=1
|__ bar
      |__ b=2
|__ c=3
```
By default root is set to `/`, and the volume will consist of the entire tree.  
If `root` is set to `/bar`, the volume will consist only of `/bar`'s children, so in this case the key `b=2` will show up as a file called `b` containing `2`  
The reamining nodes will not be visible in the volume.

**store** - *set the k/v store type to use from the supported k/v stores*

**addrs** - *comma separated list of `<address:port>` for reaching the k/v store*
