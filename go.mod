module github.com/genuinetools/img

go 1.13

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

require (
	github.com/containerd/console v1.0.3
	github.com/containerd/containerd v1.6.9
	github.com/containerd/fuse-overlayfs-snapshotter v1.0.5
	github.com/containerd/go-runc v1.0.0
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/docker/cli v20.10.12+incompatible
	github.com/docker/distribution v2.8.0+incompatible
	github.com/docker/docker v20.10.7+incompatible
	github.com/docker/go-units v0.4.0
	github.com/genuinetools/reg v0.16.0
	github.com/moby/buildkit v0.10.0
	github.com/moby/patternmatcher v0.5.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6
	github.com/opencontainers/image-spec v1.0.3-0.20211202183452-c5a74bcca799
	github.com/opencontainers/runc v1.1.2
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.1.3
	go.etcd.io/bbolt v1.3.6
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.50.1
)

replace (
	github.com/docker/cli => github.com/docker/cli v20.10.3-0.20221124184145-c0fa00e6142d+incompatible // v23.0.0-dev
	github.com/docker/docker => github.com/docker/docker v20.10.3-0.20221124164242-a913b5ad7ef1+incompatible // 22.06 branch (v23.0.0-dev)
)
