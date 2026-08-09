package dockerhealth

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Kirragami/panoptes/eye/visions"
	"github.com/Kirragami/panoptes/proto"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"
)

type dockerVigil struct {
	cancel context.CancelFunc
	done   <-chan struct{}
	once   sync.Once
}

func (v *dockerVigil) Sleep() {
	v.once.Do(func() {
		v.cancel()
		<-v.done
	})
}

func (d *DockerHealth) Awaken(
	ctx context.Context,
	gaze *proto.Gaze,
	raise visions.OmenRaiser,
) (visions.Vigil, error) {
	if raise == nil {
		return nil, fmt.Errorf("Docker Healrg has no Omen raiser")
	}

	focus, err := d.interpretFocus(gaze.GetFocus())
	if err != nil {
		return nil, err
	}

	docker, err := d.openDocker()
	if err != nil {
		return nil, err
	}

	vigilContext, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	state := newDockerState()

	var wardens sync.WaitGroup
	wardens.Add(2)

	if _, err := docker.Ping(
		vigilContext,
		client.PingOptions{
			NegotiateAPIVersion: true,
		},
	); err != nil {
		docker.Close()
		cancel()

		return nil, fmt.Errorf(
			"ping Docker Engine before Vigil: %w",
			err,
		)
	}

	go func() {
		defer wardens.Done()

		d.watchDockerEvents(
			vigilContext,
			docker,
			focus,
			gaze,
			raise,
			state,
		)
	}()

	go func() {
		defer wardens.Done()

		d.reconcileDocker(
			vigilContext,
			focus,
			gaze,
			raise,
			state,
		)
	}()

	go func() {
		wardens.Wait()
		close(done)
	}()

	return &dockerVigil{
		cancel: cancel,
		done:   done,
	}, nil
}

func (d *DockerHealth) watchDockerEvents(
	ctx context.Context,
	docker *client.Client,
	focus dockerHealthFocus,
	gaze *proto.Gaze,
	raise visions.OmenRaiser,
	state *dockerState,
) {
	defer docker.Close()

	filters := make(client.Filters)
	filters.Add("type", "container")
	filters.Add("container", focus.target)

	for {
		if ctx.Err() != nil {
			return
		}

		result := docker.Events(
			ctx,
			client.EventsListOptions{
				Filters: filters,
			},
		)

		reconnect := d.watchDockerEventStream(
			ctx,
			result,
			gaze,
			raise,
			state,
			focus.startingGracePeriod,
		)

		if !reconnect || ctx.Err() != nil {
			return
		}

		log.Printf(
			"[EYE] Docker event stream faded for %s; reconnecting",
			focus.target,
		)

		timer := time.NewTimer(time.Second)

		select {
		case <-ctx.Done():
			timer.Stop()
			return

		case <-timer.C:
		}
	}
}

func (d *DockerHealth) watchDockerEventStream(
	ctx context.Context,
	result client.EventsResult,
	gaze *proto.Gaze,
	raise visions.OmenRaiser,
	state *dockerState,
	startingGrace time.Duration,
) bool {
	for {
		select {
		case <-ctx.Done():
			return false

		case event, open := <-result.Messages:
			if !open {
				return true
			}

			switch event.Action {
			case events.ActionStart,
				events.ActionRestart:

				state.awakenNow()

			case events.ActionHealthStatusHealthy:
				state.clearOmen()

			case events.ActionStop,
				events.ActionDie,
				events.ActionDestroy,
				events.ActionKill,
				events.ActionOOM:

				d.raiseDockerOmen(
					ctx,
					gaze,
					raise,
					state,
				)

			case events.ActionHealthStatusUnhealthy:
				if state.withinGrace(startingGrace) {
					continue
				}

				d.raiseDockerOmen(
					ctx,
					gaze,
					raise,
					state,
				)
			}

		case _, open := <-result.Err:
			if !open {
				return true
			}

			return true
		}
	}
}

func (d *DockerHealth) raiseDockerOmen(
	ctx context.Context,
	gaze *proto.Gaze,
	raise visions.OmenRaiser,
	state *dockerState,
) {
	if !state.claimOmen() {
		return
	}

	omenContext, cancel := context.WithTimeout(
		ctx,
		5*time.Second,
	)
	defer cancel()

	if err := raise(omenContext, gaze); err != nil {
		state.releaseOmen()
		log.Printf(
			"[EYE] Failed to raise Omen for Gaze %s: %v",
			gaze.GetSigil(),
			err,
		)
	}
}
