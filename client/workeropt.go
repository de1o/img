package client

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/runc/libcontainer/user"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/diff/apply"
	"github.com/containerd/containerd/diff/walking"
	ctdmetadata "github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/platforms"
	ctdsnapshot "github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/native"
	"github.com/containerd/containerd/snapshots/overlay"
	fuseoverlayfs "github.com/containerd/fuse-overlayfs-snapshotter"
	"github.com/genuinetools/img/types"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/executor"
	executoroci "github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/executor/runcexecutor"
	containerdsnapshot "github.com/moby/buildkit/snapshot/containerd"
	"github.com/moby/buildkit/util/archutil"
	"github.com/moby/buildkit/util/leaseutil"
	"github.com/moby/buildkit/util/network/netproviders"
	"github.com/moby/buildkit/worker/base"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

// GetParentNSeuid returns the euid within the parent user namespace
func GetParentNSeuid() int64 {
	euid := int64(os.Geteuid())
	uidmap, err := user.CurrentProcessUIDMap()
	if err != nil {
		// This kernel-provided file only exists if user namespaces are supported
		return euid
	}
	for _, um := range uidmap {
		if um.ID <= euid && euid <= um.ID+um.Count-1 {
			return um.ParentID + euid - um.ID
		}
	}
	return euid
}

func (c *Client) createWorkerOpt(withExecutor bool) (opt base.WorkerOpt, err error) {
	return c.createWorkerOptInner(withExecutor, false, 0)
}

// createWorkerOpt creates a base.WorkerOpt to be used for a new worker.
func (c *Client) createWorkerOptInner(withExecutor bool, insecure bool, unprivilegedFlag int) (opt base.WorkerOpt, err error) {
	// Create the metadata store.
	logrus.Printf("Creating the metadata store...\n")
	md, err := metadata.NewStore(filepath.Join(c.root, "metadata.db"))
	if err != nil {
		return opt, err
	}

	snapshotRoot := filepath.Join(c.root, "snapshots")
	var unprivileged bool
	if unprivilegedFlag == 0 {
		unprivileged = GetParentNSeuid() != 0
	} else if unprivilegedFlag == 1 {
		unprivileged = true
	} else {
		unprivileged = false
	}
	logrus.Printf("Creating the snapshotter...\n")

	// Create the snapshotter.
	var (
		s ctdsnapshot.Snapshotter
	)
	switch c.backend {
	case types.NativeBackend:
		s, err = native.NewSnapshotter(snapshotRoot)
	case types.OverlayFSBackend:
		// On some distros such as Ubuntu overlayfs can be mounted without privileges
		s, err = overlay.NewSnapshotter(snapshotRoot)
	case types.FUSEOverlayFSBackend:
		s, err = fuseoverlayfs.NewSnapshotter(snapshotRoot)
	default:
		// "auto" backend needs to be already resolved on Client instantiation
		return opt, fmt.Errorf("%s is not a valid snapshots backend", c.backend)
	}
	if err != nil {
		return opt, fmt.Errorf("creating %s snapshotter failed: %v", c.backend, err)
	}
	logrus.Printf("Created the snapshotter...\n")

	var exe executor.Executor
	if withExecutor {
		exeOpt := runcexecutor.Opt{
			Root:        filepath.Join(c.root, "executor"),
			Rootless:    unprivileged,
			ProcessMode: processMode(),
		}

		np, _, err := netproviders.Providers(netproviders.Opt{Mode: "auto"})
		if err != nil {
			return base.WorkerOpt{}, err
		}

		exe, err = runcexecutor.New(exeOpt, np)
		if err != nil {
			return opt, err
		}
		logrus.Printf("Created the executor...\n")
	}

	// Create the content store locally.
	contentStore, err := local.NewStore(filepath.Join(c.root, "content"))
	if err != nil {
		return opt, err
	}
	logrus.Printf("Created the content store...\n")

	// Open the bolt database for metadata.
	db, err := bolt.Open(filepath.Join(c.root, "containerdmeta.db"), 0644, nil)
	if err != nil {
		return opt, err
	}
	logrus.Printf("Opened the bolt database...\n")

	// Create the new database for metadata.
	mdb := ctdmetadata.NewDB(db, contentStore, map[string]ctdsnapshot.Snapshotter{
		c.backend: s,
	})
	if err := mdb.Init(context.TODO()); err != nil {
		return opt, err
	}
	logrus.Printf("Created the metadata database...\n")

	// Create the image store.
	imageStore := ctdmetadata.NewImageStore(mdb)
	logrus.Printf("Created the image store...\n")

	contentStore = containerdsnapshot.NewContentStore(mdb.ContentStore(), "buildkit")
	logrus.Printf("Created the content store...\n")

	id, err := base.ID(c.root)
	if err != nil {
		return opt, err
	}

	// format xlabels as map
	xlabels := map[string]string{
		"oci": c.backend,
	}

	var supportedPlatforms []specs.Platform
	for _, p := range archutil.SupportedPlatforms(false) {
		supportedPlatforms = append(supportedPlatforms, platforms.Normalize(p))
	}

	registriesHosts := docker.ConfigureDefaultRegistries()
	if insecure {
		registriesHosts = configureRegistries("http")
	}

	opt = base.WorkerOpt{
		ID:             id,
		Labels:         xlabels,
		MetadataStore:  md,
		Executor:       exe,
		Snapshotter:    containerdsnapshot.NewSnapshotter(c.backend, mdb.Snapshotter(c.backend), "buildkit", nil),
		ContentStore:   contentStore,
		Applier:        apply.NewFileSystemApplier(contentStore),
		Differ:         walking.NewWalkingDiff(contentStore),
		ImageStore:     imageStore,
		Platforms:      supportedPlatforms,
		RegistryHosts:  registriesHosts,
		LeaseManager:   leaseutil.WithNamespace(ctdmetadata.NewLeaseManager(mdb), "buildkit"),
		GarbageCollect: mdb.GarbageCollect,
	}
	logrus.Printf("Created the worker options...\n")

	return opt, err
}

func processMode() executoroci.ProcessMode {
	mountArgs := []string{"-t", "proc", "none", "/proc"}
	cmd := exec.Command("mount", mountArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig:    syscall.SIGKILL,
		Cloneflags:   syscall.CLONE_NEWPID,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	if b, err := cmd.CombinedOutput(); err != nil {
		logrus.Warnf("Process sandbox is not available, consider unmasking procfs: %v", string(b))
		return executoroci.NoProcessSandbox
	}
	return executoroci.ProcessSandbox
}
