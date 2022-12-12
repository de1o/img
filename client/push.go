package client

import (
	"context"
	"fmt"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strings"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/util/push"
)

// Push sends an image to a remote registry.
func (c *Client) Push(ctx context.Context, image string, insecure bool, sessionId string) error {
	logrus.Printf("Entering Pushing %s...\n", image)
	// Parse the image name and tag.
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return fmt.Errorf("parsing image name %q failed: %v", image, err)
	}
	// Add the latest lag if they did not provide one.
	named = reference.TagNameOnly(named)
	image = named.String()

	// Create the worker opts.
	opt, err := c.createWorkerOpt(false)
	if err != nil {
		return fmt.Errorf("creating worker opt failed: %v", err)
	}
	logrus.Printf("worker opt created...\n")

	imgObj, err := opt.ImageStore.Get(ctx, image)
	if err != nil {
		return fmt.Errorf("getting image %q failed: %v", image, err)
	}
	logrus.Printf("image store created...\n")

	sm, err := c.getSessionManager()
	if err != nil {
		return err
	}

	// ctx context.Context, sm *session.Manager, sid string, provider content.Provider, manager content.Manager,
	// dgst digest.Digest, ref string, insecure bool, hosts docker.RegistryHosts, byDigest bool, annotations map[digest.Digest]map[string]string
	logrus.Printf("Try to push image %s...\n", image)
	if err := push.Push(ctx, sm, sessionId,
		contentutil.NewMultiProvider(opt.ContentStore),
		opt.ContentStore,
		imgObj.Target.Digest, image, insecure, opt.RegistryHosts, false, nil); err != nil {
		logrus.Printf("Push image %s failed: %v\n", image, err)
		if !isErrHTTPResponseToHTTPSClient(err) {
			return errors.Wrapf(err, "not http response to https client")
		}

		if !insecure {
			return errors.Wrapf(err, "push failed, try --insecure")
		}

		return push.Push(ctx, sm, sessionId, contentutil.NewMultiProvider(opt.ContentStore), opt.ContentStore, imgObj.Target.Digest, image, insecure, registryHostsWithPlainHTTP(), false, nil)
	}
	return nil
}

func isErrHTTPResponseToHTTPSClient(err error) bool {
	// The error string is unexposed as of Go 1.13, so we can't use `errors.Is`.
	// https://github.com/golang/go/issues/44855

	const unexposed = "server gave HTTP response to HTTPS client"
	const refuesd = "connection refused"
	return strings.Contains(err.Error(), unexposed) || strings.Contains(err.Error(), refuesd)
}

func registryHostsWithPlainHTTP() docker.RegistryHosts {
	logrus.Printf("registry hosts with plain HTTP...\n")
	return docker.ConfigureDefaultRegistries(docker.WithPlainHTTP(func(_ string) (bool, error) {
		return true, nil
	}))
}
