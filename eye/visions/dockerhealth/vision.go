package dockerhealth

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/client"
)

const (
	sight               = "docker.health"
	form                = 1
	defaultDockerSocket = "/var/run/docker.sock"
)

type DockerHealth struct {
	socket string
}

func New(socket string) *DockerHealth {
	if socket == "" {
		socket = defaultDockerSocket
	}

	return &DockerHealth{
		socket: socket,
	}
}

func (d *DockerHealth) Sight() string {
	return sight
}

func (d *DockerHealth) Form() uint32 {
	return form
}

func (d * DockerHealth) openDocker() (
	*client.Client,
	error,
) {
	docker, err := client.New(
		client.WithHost("unix://" + d.socket),
	)
	if err != nil {
		return nil, fmt.Errorf("open Docker Engine: %w", err)
	}

	return docker, nil
}

func (d *DockerHealth) Behold(
	ctx context.Context,
) (awake bool, slumberReason string, err error) {
	probeContext, cancel := context.WithTimeout(
		ctx,
		2*time.Second,
	)
	defer cancel()

	docker, err := d.openDocker()
	if err != nil {
		return false, "Docker Engine cannot be opened", nil
	}
	defer docker.Close()

	if _, err := docker.Ping(
		probeContext,
		client.PingOptions{
			NegotiateAPIVersion: true,
		},
	); err != nil {
		return false, "Docker Engine cannot be pinged", nil
	}

	return true, "", nil
}
