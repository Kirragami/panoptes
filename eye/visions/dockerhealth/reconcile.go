package dockerhealth

import (
	"context"
	"time"

	"github.com/Kirragami/panoptes/eye/visions"
	"github.com/Kirragami/panoptes/proto"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func (d *DockerHealth) reconcileDocker(
	ctx context.Context,
	focus dockerHealthFocus,
	gaze *proto.Gaze,
	raise visions.OmenRaiser,
	state *dockerState,
) {
	d.reconcileDockerOnce(
		ctx,
		focus,
		gaze,
		raise,
		state,
	)

	ticker := time.NewTicker(focus.reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			d.reconcileDockerOnce(
				ctx,
				focus,
				gaze,
				raise,
				state,
			)
		}
	}
}

func (d *DockerHealth) reconcileDockerOnce(
	ctx context.Context,
	focus dockerHealthFocus,
	gaze *proto.Gaze,
	raise visions.OmenRaiser,
	state *dockerState,
) {
	docker, err := d.openDocker()
	if err != nil {
		d.raiseDockerOmen(ctx, gaze, raise, state)
		return
	}
	defer docker.Close()

	inspectionContext, cancel := context.WithTimeout(
		ctx,
		5*time.Second,
	)
	defer cancel()

	inspection, err := docker.ContainerInspect(
		inspectionContext,
		focus.target,
		client.ContainerInspectOptions{},
	)
	if err != nil {
		d.raiseDockerOmen(ctx, gaze, raise, state)
		return
	}

	containerState := inspection.Container.State
	if containerState == nil {
		d.raiseDockerOmen(ctx, gaze, raise, state)
		return
	}

	if !containerState.Running {
		if containerState.Restarting &&
			state.withinGrace(focus.startingGracePeriod) {
			return
		}

		d.raiseDockerOmen(ctx, gaze, raise, state)
		return
	}

	if containerState.Health != nil &&
		containerState.Health.Status == container.Unhealthy {
		if state.withinGrace(focus.startingGracePeriod) {
			return
		}

		d.raiseDockerOmen(ctx, gaze, raise, state)
		return
	}

	state.clearOmen()
}