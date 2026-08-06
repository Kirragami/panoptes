package visions

import (
	"context"
	"net"
	"time"
)

const (
	dockerHealthSight = "docker.health"
	dockerHealthForm = 1
	defaultDockerSocket = "/var/run/docker.sock"
)

type DockerHealth struct {
	socket string
}

func NewDockerHealth(socket string) *DockerHealth {
	if socket == "" {
		socket = defaultDockerSocket
	}

	return &DockerHealth{
		socket: socket,
	}
}

func (d *DockerHealth) Sight() string {
	return dockerHealthSight
}

func (d *DockerHealth) Form() uint32 {
	return dockerHealthForm
}

func (d *DockerHealth) Behold(
	ctx context.Context,
) (awake bool, slumberReason string, err error) {
	probeContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	connection, err := (&net.Dialer{}).DialContext(
		probeContext,
		"unix",
		d.socket,
	)
	if err != nil {
		return false, "Docker socket cannot be reached", nil
	}
	defer connection.Close()

	return true, "", nil
}