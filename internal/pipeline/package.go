package pipeline

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

type Task struct {
	Image        string        `yaml:"Image"`
	Command      []string      `yaml:"Command"`
	VolumeMounts []VolumeMount `yaml:"VolumeMounts"`
}

type Volume struct {
	Name     string `yaml:"Name"`
	HostPath string `yaml:"HostPath"`
}

type VolumeMount struct {
	Name          string `yaml:"Name"`
	ContainerPath string `yaml:"ContainerPath"`
}

type Manifest struct {
	Tasks   []Task   `yaml:"Tasks"`
	Volumes []Volume `yaml:"Volumes"`
}

type Pipeline struct {
	tasks   []Task
	volumes []Volume
}

func NewPipeline(manifest Manifest) Pipeline {
	return Pipeline{
		tasks:   manifest.Tasks,
		volumes: manifest.Volumes,
	}
}

func (pipeline *Pipeline) Execute(args []string) error {
	var err error

	var cli *client.Client
	if cli, err = client.NewClientWithOpts(client.FromEnv); err != nil {
		return fmt.Errorf("failed to create docker client: %v", err)
	}

	ctx := context.Background()

	var pwd string
	if pwd, err = os.Getwd(); err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}

	for _, task := range pipeline.tasks {
		imagePullOptions := types.ImagePullOptions{}

		var reader io.ReadCloser
		if reader, err = cli.ImagePull(ctx, task.Image, imagePullOptions); err != nil {
			return fmt.Errorf("failed to pull image: %v", err)
		}

		defer reader.Close()
		io.Copy(os.Stdout, reader)

		config := container.Config{
			Image: task.Image,
			Cmd:   task.Command,
		}

		var mounts []mount.Mount
		for _, volumeMount := range task.VolumeMounts {
			var volume *Volume
			for _, v := range pipeline.volumes {
				if v.Name == volumeMount.Name {
					volume = &v
					break
				}
			}

			if volume == nil {
				return fmt.Errorf("failed to find volume: %v", volumeMount.Name)
			}

			hostPath := path.Join(pwd, volume.HostPath)
			if err = os.MkdirAll(hostPath, 0755); err != nil {
				return fmt.Errorf("failed to create volume: %v", volumeMount.Name)
			}

			mount := mount.Mount{
				Type:   mount.TypeBind,
				Source: hostPath,
				Target: volumeMount.ContainerPath,
			}

			mounts = append(mounts, mount)
		}

		hostConfig := container.HostConfig{
			Mounts: mounts,
		}

		var response container.ContainerCreateCreatedBody
		if response, err = cli.ContainerCreate(ctx, &config, &hostConfig, nil, nil, ""); err != nil {
			return fmt.Errorf("failed to create container: %v", err)
		}

		containerStartOptions := types.ContainerStartOptions{}
		if err = cli.ContainerStart(ctx, response.ID, containerStartOptions); err != nil {
			return fmt.Errorf("failed to start container: %v", err)
		}

		if err = waitForCompletion(cli, ctx, response); err != nil {
			return fmt.Errorf("failed to wait for container: %v", err)
		}

		containerRemoveOptions := types.ContainerRemoveOptions{}
		if err = cli.ContainerRemove(ctx, response.ID, containerRemoveOptions); err != nil {
			log.Printf("[warning] failed to remove container: %v\n", err)
		}
	}

	return nil
}

func waitForCompletion(cli *client.Client, ctx context.Context, response container.ContainerCreateCreatedBody) error {
	status, errs := cli.ContainerWait(ctx, response.ID, container.WaitConditionNotRunning)

	select {
	case err := <-errs:
		return err
	case <-status:
		return nil
	}
}
