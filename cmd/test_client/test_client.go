package main

import (
	"context"
	"log"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
)

const ref = "/home/dhenderson/work/perf_testing_experiment/cmd/test_client/docker/stress-ng-testing"

func redisExample() error {
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		return err
	}
	defer client.Close()
	ctx := namespaces.WithNamespace(context.Background(), "stress-ng-testing")
	image, err := client.Pull(ctx, ref, containerd.WithPullUnpack)
	if err != nil {
		if image, err = client.GetImage(ctx, ref); err != nil {
			return err
		}
	}
	log.Printf("Successfully pulled %s image\n", image.Name())
	container, err := getContainer(client, ctx, image)
	if err != nil {
		return err
	}
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}
	defer task.Delete(ctx)
	service, _ := client.Server(ctx)
	log.Println(service)
	log.Println(client.Containers(ctx))
	return nil
}
func getContainer(client *containerd.Client, ctx context.Context, image containerd.Image) (containerd.Container, error) {
	var jack []containerd.Container
	c, err := client.NewContainer(
		ctx,
		"stress-ng-testing",
		containerd.WithSnapshot("stress-ng-testing-snapshot"),
		containerd.WithNewSpec(oci.WithImageConfig(image)),
	)
	if err != nil {
		jack, _ = client.Containers(ctx)
	}
	jack[0].Delete(ctx)
	if len(jack) > 0 {
		c = jack[0]
	}
	return c, err
}
func main() {
	log.Println(redisExample())
	return
}
