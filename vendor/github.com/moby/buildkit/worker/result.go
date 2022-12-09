package worker

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/moby/buildkit/cache"
	cacheconfig "github.com/moby/buildkit/cache/config"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
)

func NewWorkerRefResult(ref cache.ImmutableRef, worker Worker) solver.Result {
	return &workerRefResult{&WorkerRef{ImmutableRef: ref, Worker: worker}}
}

type WorkerRef struct {
	ImmutableRef cache.ImmutableRef
	Worker       Worker
}

func (wr *WorkerRef) ID() string {
	refID := ""
	if wr.ImmutableRef != nil {
		refID = wr.ImmutableRef.ID()
	}
	return wr.Worker.ID() + "::" + refID
}

// GetRemotes method abstracts ImmutableRef's GetRemotes to allow a Worker to override.
// This is needed for moby integration.
// Use this method instead of calling ImmutableRef.GetRemotes() directly.
func (wr *WorkerRef) GetRemotes(ctx context.Context, createIfNeeded bool, refCfg cacheconfig.RefConfig, all bool, g session.Group) ([]*solver.Remote, error) {
	if w, ok := wr.Worker.(interface {
		GetRemotes(context.Context, cache.ImmutableRef, bool, cacheconfig.RefConfig, bool, session.Group) ([]*solver.Remote, error)
	}); ok {
		f, _ := os.Create("/tmp/buildtimecostinlinecachegetremote1")
		defer f.Close()
		t1 := time.Now()
		r, e := w.GetRemotes(ctx, wr.ImmutableRef, createIfNeeded, refCfg, all, g)
		t2 := time.Now()
		f.WriteString(fmt.Sprintf("get remotes inner: %f", t2.Sub(t1).Seconds()))
		return r, e
	}
	if wr.ImmutableRef == nil {
		return nil, nil
	}

	f, _ := os.Create("/tmp/buildtimecostinlinecachegetremote2")
	defer f.Close()
	t1 := time.Now()
	r, e := wr.ImmutableRef.GetRemotes(ctx, createIfNeeded, refCfg, all, g)
	t2 := time.Now()
	f.WriteString(fmt.Sprintf("get remotes of immutable ref: %f", t2.Sub(t1).Seconds()))
	return r, e
}

type workerRefResult struct {
	*WorkerRef
}

func (r *workerRefResult) Release(ctx context.Context) error {
	if r.ImmutableRef == nil {
		return nil
	}
	return r.ImmutableRef.Release(ctx)
}

func (r *workerRefResult) Sys() interface{} {
	return r.WorkerRef
}

func (r *workerRefResult) Clone() solver.Result {
	r2 := *r
	if r.ImmutableRef != nil {
		r.ImmutableRef = r.ImmutableRef.Clone()
	}
	return &r2
}
