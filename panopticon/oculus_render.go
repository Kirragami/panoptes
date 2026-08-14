package main

import (
	"fmt"
	"math"
	"strings"
)

// Strict binary palette — camera→monitor collapses cream/yellow/red.
const (
	oculusInk   = "#000000"
	oculusPaper = "#ffffff"
)

func oculusSectorPath(cx, cy, r0, r1, a0, a1 float64) string {
	point := func(r, a float64) (float64, float64) {
		return cx + r*math.Sin(a), cy - r*math.Cos(a)
	}

	x0, y0 := point(r1, a0)
	x1, y1 := point(r1, a1)
	x2, y2 := point(r0, a1)
	x3, y3 := point(r0, a0)

	large := 0
	if a1-a0 > math.Pi {
		large = 1
	}

	return fmt.Sprintf(
		"M %.2f %.2f A %.2f %.2f 0 %d 1 %.2f %.2f L %.2f %.2f A %.2f %.2f 0 %d 0 %.2f %.2f Z",
		x0, y0, r1, r1, large, x1, y1, x2, y2, r0, r0, large, x3, y3,
	)
}

// Inverted rim eye (white / black / white). Used for NW, NE, SW.
func oculusCircularEyeFinder(x, y float64) string {
	module := oculusFinderModule
	rOuter := 3.5 * module
	rDark := 2.5 * module
	rCore := 1.5 * module
	quiet := oculusFinderQuietR()
	return fmt.Sprintf(
		`<g>`+
			`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
			`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
			`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
			`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
			`</g>`,
		x, y, quiet, oculusInk,
		x, y, rOuter, oculusPaper,
		x, y, rDark, oculusInk,
		x, y, rCore, oculusPaper,
	)
}

// Classic dark-core eye for SE — structural orientation (luma only).
// White halo keeps B/W/B ratios from merging into the black field.
func oculusClassicEyeFinder(x, y float64) string {
	module := oculusFinderModule
	rHalo := 4.6 * module
	rOuter := 3.5 * module
	rMid := 2.5 * module
	rCore := 1.5 * module
	return fmt.Sprintf(
		`<g>`+
			`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
			`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
			`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
			`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`+
			`</g>`,
		x, y, rHalo, oculusPaper,
		x, y, rOuter, oculusInk,
		x, y, rMid, oculusPaper,
		x, y, rCore, oculusInk,
	)
}

func renderOculusSVG(oracleSeal string) (string, error) {
	bits, err := encodeOculusPayload(oracleSeal)
	if err != nil {
		return "", err
	}

	cx := oculusCenterX
	cy := oculusCenterY

	var svg strings.Builder
	svg.Grow(128 * 1024)

	fmt.Fprintf(
		&svg,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" role="img" aria-label="Oracle Seal oculus" shape-rendering="crispEdges">`,
		oculusViewSize,
		oculusViewSize,
	)

	// Solid black field covering rim finders.
	fieldR := oculusFinderOrbitR() + oculusFinderQuietR() + 12
	fmt.Fprintf(
		&svg,
		`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`,
		cx, cy, fieldR, oculusInk,
	)

	// Slightly tighter gaps than the cream era — more ink for camera sampling.
	bitIndex := 0
	gap := 0.012
	for ring := 0; ring < oculusRingCount; ring++ {
		r0 := oculusInnerR + float64(ring)*oculusRingWidth
		r1 := r0 + oculusRingWidth
		r0i := r0 + 1.5
		r1i := r1 - 1.5
		sectors := oculusSectorsPerRing[ring]
		step := 2 * math.Pi / float64(sectors)

		for sector := 0; sector < sectors; sector++ {
			var bit byte

			switch {
			case ring == oculusTimingRingIndex:
				// v7: double-width paper wedge at north (sectors 0+1), then alternate.
				if sector == 0 {
					a0 := gap
					a1 := float64(oculusTimingZeroWidth)*step - gap
					path := oculusSectorPath(cx, cy, r0i, r1i, a0, a1)
					fmt.Fprintf(&svg, `<path d="%s" fill="%s"/>`, path, oculusPaper)
					continue
				}
				if sector < oculusTimingZeroWidth {
					continue
				}
				bit = byte(sector % 2)
			case oculusIsPhaseLock(ring, sector):
				// Drawn later as donut locks.
				continue
			default:
				if bitIndex >= len(bits) {
					return "", errOculus("oculus renderer ran out of petal bits")
				}
				bit = bits[bitIndex]
				bitIndex++
			}

			if bit == 0 {
				continue
			}

			a0 := float64(sector)*step + gap
			a1 := float64(sector+1)*step - gap
			path := oculusSectorPath(cx, cy, r0i, r1i, a0, a1)
			fmt.Fprintf(&svg, `<path d="%s" fill="%s"/>`, path, oculusPaper)
		}
	}

	if bitIndex != len(bits) {
		return "", errOculus("oculus renderer did not consume every petal bit")
	}

	// Phase locks: white donut (paper ring + ink pip) — distinct from solid petals.
	for ring := 0; ring < oculusRingCount; ring++ {
		if ring == oculusTimingRingIndex {
			continue
		}
		r0 := oculusInnerR + float64(ring)*oculusRingWidth
		r1 := r0 + oculusRingWidth
		r0i := r0 + 1.5
		r1i := r1 - 1.5
		sectors := oculusSectorsPerRing[ring]
		step := 2 * math.Pi / float64(sectors)
		for _, angle := range oculusPhaseLockAngles {
			sector := oculusAngleToSector(angle, sectors)
			a0 := float64(sector)*step + gap*0.35
			a1 := float64(sector+1)*step - gap*0.35
			fmt.Fprintf(
				&svg,
				`<path d="%s" fill="%s"/>`,
				oculusSectorPath(cx, cy, r0i, r1i, a0, a1),
				oculusPaper,
			)
			midR := (r0 + r1) / 2.0
			midA := (float64(sector) + 0.5) * step
			px, py := oculusPolar(midR, midA)
			pip := math.Min(oculusRingWidth*0.28, (a1-a0)*midR*0.38)
			fmt.Fprintf(
				&svg,
				`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`,
				px, py, pip, oculusInk,
			)
		}
	}

	// Outer timing ring in the quiet rim.
	ot0 := oculusOuterR + 8
	ot1 := oculusOuterR + 30
	otStep := 2 * math.Pi / float64(oculusOuterTimingSectors)
	for sector := 0; sector < oculusOuterTimingSectors; sector++ {
		if sector%2 == 0 {
			continue
		}
		a0 := float64(sector)*otStep + 0.02
		a1 := float64(sector+1)*otStep - 0.02
		fmt.Fprintf(
			&svg,
			`<path d="%s" fill="%s"/>`,
			oculusSectorPath(cx, cy, ot0, ot1, a0, a1),
			oculusPaper,
		)
	}

	// Cardinal axis ticks only (no colored spokes through data petals).
	for k := 0; k < 4; k++ {
		ang := float64(k) * (math.Pi / 2)
		x0, y0 := oculusPolar(oculusOuterR+4, ang)
		x1, y1 := oculusPolar(oculusOuterR+30, ang)
		fmt.Fprintf(
			&svg,
			`<path d="M %.2f %.2f L %.2f %.2f" stroke="%s" stroke-width="4" stroke-linecap="square"/>`,
			x0, y0, x1, y1, oculusPaper,
		)
	}

	// Angle-zero crown (white structural north mark).
	fmt.Fprintf(
		&svg,
		`<path d="M %.2f %.2f L %.2f %.2f" stroke="%s" stroke-width="10" stroke-linecap="square"/>`,
		cx, cy-oculusOuterR-34, cx, cy-oculusOuterR+8, oculusPaper,
	)
	fmt.Fprintf(
		&svg,
		`<circle cx="%.2f" cy="%.2f" r="8" fill="%s"/>`,
		cx, cy-oculusOuterR-38, oculusPaper,
	)

	// Flat black pupil — no grey/red ornament.
	pupilR := oculusInnerR - 10
	fmt.Fprintf(
		&svg,
		`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`,
		cx, cy, pupilR, oculusInk,
	)
	fmt.Fprintf(
		&svg,
		`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="none" stroke="%s" stroke-width="6"/>`,
		cx, cy, pupilR-4, oculusPaper,
	)

	finders := oculusInCircleFinders()
	for i, finder := range finders {
		if i == 2 {
			svg.WriteString(oculusClassicEyeFinder(finder[0], finder[1]))
			continue
		}
		svg.WriteString(oculusCircularEyeFinder(finder[0], finder[1]))
	}

	svg.WriteString(`</svg>`)
	return svg.String(), nil
}
