package main

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// https://stackoverflow.com/questions/45805563/pull-a-file-from-a-docker-image-in-golang-to-local-file-system
// https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07
func pullImage(imageName string) error {
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	img, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	defer img.Close()
	if _, err := io.ReadAll(img); err != nil {
		return err
	}
	return nil
}
