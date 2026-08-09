package dockerhealth

import (
	"fmt"
	"math"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

const (
	defaultReconcileSeconds = 60
	defaultStartingGrace    = 120
)

type dockerHealthFocus struct {
	target              string
	reconcileInterval   time.Duration
	startingGracePeriod time.Duration
}

func (d *DockerHealth) DiscernFocus(
	focus *structpb.Struct,
) error {
	_, err := d.interpretFocus(focus)
	return err
}

func (d *DockerHealth) interpretFocus(
	focus *structpb.Struct,
) (dockerHealthFocus, error) {
	if focus == nil {
		return dockerHealthFocus{}, fmt.Errorf(
			"Docker Health Gaze has no focus",
		)
	}

	fields := focus.GetFields()

	allowed := map[string]struct{}{
		"target":                     {},
		"reconcile_interval_seconds": {},
		"starting_grace_seconds":     {},
	}

	for name := range fields {
		if _, accepted := allowed[name]; !accepted {
			return dockerHealthFocus{}, fmt.Errorf(
				"Docker Health does not understand the focus %q",
				name,
			)
		}
	}

	target, exists := fields["target"]
	if !exists || strings.TrimSpace(target.GetStringValue()) == "" {
		return dockerHealthFocus{}, fmt.Errorf(
			"Docker Health requires a non-empty target",
		)
	}

	reconcileSeconds, err := focusWholeNumber(
		fields,
		"reconcile_interval_seconds",
		defaultReconcileSeconds,
		1,
		86400,
	)
	if err != nil {
		return dockerHealthFocus{}, err
	}

	graceSeconds, err := focusWholeNumber(
		fields,
		"starting_grace_seconds",
		defaultStartingGrace,
		0,
		86400,
	)
	if err != nil {
		return dockerHealthFocus{}, err
	}

	return dockerHealthFocus{
		target:              strings.TrimSpace(target.GetStringValue()),
		reconcileInterval:   time.Duration(reconcileSeconds) * time.Second,
		startingGracePeriod: time.Duration(graceSeconds) * time.Second,
	}, nil
}

func focusWholeNumber(
	fields map[string]*structpb.Value,
	name string,
	fallback int,
	minimum float64,
	maximum float64,
) (int, error) {
	value, exists := fields[name]
	if !exists {
		return fallback, nil
	}
	number, isNumber := value.Kind.(*structpb.Value_NumberValue)
	if !isNumber {
		return 0, fmt.Errorf("%s must be a number", name)
	}
	if math.Trunc(number.NumberValue) != number.NumberValue {
		return 0, fmt.Errorf("%s must be a whole number", name)
	}
	if number.NumberValue < minimum || number.NumberValue > maximum {
		return 0, fmt.Errorf(
			"%s must be between %.0f and %.0f",
			name,
			minimum,
			maximum,
		)
	}
	return int(number.NumberValue), nil
}
