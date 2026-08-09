package visions

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

const (
	dockerHealthSight   = "docker.health"
	dockerHealthForm    = 1
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

func (d *DockerHealth) DiscernFocus(
	focus *structpb.Struct,
) error {
	if focus == nil {
		return fmt.Errorf("Docker Health Gaze has no focus")
	}

	fields := focus.GetFields()

	allowed := map[string]struct{}{
		"target":                     {},
		"reconcile_interval_seconds": {},
		"starting_grace_seconds":     {},
	}

	for name := range fields {
		if _, accepted := allowed[name]; !accepted {
			return fmt.Errorf(
				"Docker Health does not understand focus %q",
				name,
			)
		}
	}

	target, exists := fields["target"]
	if !exists || strings.TrimSpace(target.GetStringValue()) == "" {
		return fmt.Errorf(
			"Docker Health requires a non-empty target",
		)
	}

	if err := discernWholeNumber(
		fields,
		"reconcile_interval_seconds",
		1,
		86400,
	); err != nil {
		return err
	}
	if err := discernWholeNumber(
		fields,
		"starting_grace_seconds",
		0,
		86400,
	); err != nil {
		return err
	}
	return nil
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

func discernWholeNumber(
	fields map[string]*structpb.Value,
	name string,
	minimum float64,
	maximum float64,
) error {
	value, exists := fields[name]
	if !exists {
		return nil
	}
	number, isNumber := value.Kind.(*structpb.Value_NumberValue)
	if !isNumber {
		return fmt.Errorf("%s must be a number", name)
	}
	if math.Trunc(number.NumberValue) != number.NumberValue {
		return fmt.Errorf("%s must be a whole number", name)
	}
	if number.NumberValue < minimum || number.NumberValue > maximum {
		return fmt.Errorf(
			"%s must be between %.0f and %.0f",
			name,
			minimum,
			maximum,
		)
	}
	return nil
}
