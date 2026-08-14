package main

import "math"

const (
	// v7 finders need a slightly larger canvas than v6's 1400.
	oculusViewSize = 1480.0

	oculusCenterX = 740.0
	oculusCenterY = 752.5

	// v7: thicker rings, larger rim finders, absolute timing-zero mark.
	oculusRingWidth = 54.0
	oculusInnerR    = 88.0
	oculusRingCount = 8
	oculusOuterR    = oculusInnerR + float64(oculusRingCount)*oculusRingWidth
	oculusQuiet     = 36.0

	oculusTimingRingIndex = 0
	// Double-width paper wedge at north (sectors 0+1) for absolute angle.
	oculusTimingZeroWidth = 2

	// Larger bullseyes survive camera downscale / moiré better than v6's 14.
	oculusFinderModule = 18.0

	oculusSpokeCount         = 8
	oculusOuterTimingSectors = 48

	oculusVersion = 7
	oculusRSK     = 37
)

var oculusSectorsPerRing = []int{
	// ring0 = inner timing (excluded from payload)
	16,
	// data rings — payload excludes phase locks only
	48, 52, 56, 60, 64, 68, 64,
}

// Absolute angles (0 = north) for N/E/S/W phase-lock petals.
var oculusPhaseLockAngles = []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2}

func oculusFinderOuterR() float64 {
	return 3.5 * oculusFinderModule
}

func oculusFinderQuietR() float64 {
	return oculusFinderOuterR() + oculusFinderModule*0.55
}

func oculusFinderOrbitR() float64 {
	// Extra clearance past outer timing so eyes sit on flat black.
	return oculusOuterR + 42 + oculusFinderOuterR() + 8
}

func oculusPolar(radius, angle float64) (float64, float64) {
	return oculusCenterX + radius*math.Sin(angle),
		oculusCenterY - radius*math.Cos(angle)
}

func oculusInCircleFinders() [][2]float64 {
	r := oculusFinderOrbitR()
	// All four corners — live tilt needs 4 real correspondences (QR-style).
	angles := []float64{-math.Pi / 4, math.Pi / 4, 3 * math.Pi / 4, -3 * math.Pi / 4} // NW, NE, SE, SW
	out := make([][2]float64, len(angles))
	for i, a := range angles {
		x, y := oculusPolar(r, a)
		out[i] = [2]float64{x, y}
	}
	return out
}

func oculusInCircleOrientation() [2]float64 {
	return oculusInCircleFinders()[2] // SE bullseye (classic dark-core, B&W)
}

// Corners in cyclic order NW, NE, SE, SW (rotated square / diamond).
func oculusCorners() [][2]float64 {
	return oculusInCircleFinders()
}

func oculusAngleToSector(angle float64, sectors int) int {
	step := 2 * math.Pi / float64(sectors)
	raw := math.Floor(angle / step)
	s := int(raw) % sectors
	if s < 0 {
		s += sectors
	}
	return s
}

func oculusPetalCenter(ring, sector int) (float64, float64) {
	sectors := oculusSectorsPerRing[ring]
	step := 2 * math.Pi / float64(sectors)
	angle := (float64(sector) + 0.5) * step
	radius := oculusInnerR + float64(ring)*oculusRingWidth + oculusRingWidth*0.5
	return oculusPolar(radius, angle)
}

func oculusIsPhaseLock(ring, sector int) bool {
	if ring == oculusTimingRingIndex {
		return false
	}
	n := oculusSectorsPerRing[ring]
	for _, angle := range oculusPhaseLockAngles {
		if oculusAngleToSector(angle, n) == sector {
			return true
		}
	}
	return false
}

// oculusTimingSectorIsLight reports the v7 timing pattern (wide north + alternate).
func oculusTimingSectorIsLight(sector int) bool {
	if sector < 0 {
		return false
	}
	if sector < oculusTimingZeroWidth {
		return true
	}
	return sector%2 == 1
}

// oculusSectorCarriesPayload reports whether this petal stores an RS bit.
func oculusSectorCarriesPayload(ring, sector int) bool {
	if ring == oculusTimingRingIndex {
		return false
	}
	if oculusIsPhaseLock(ring, sector) {
		return false
	}
	return true
}

func oculusPayloadBits() int {
	total := 0
	for ring := 0; ring < oculusRingCount; ring++ {
		for sector := 0; sector < oculusSectorsPerRing[ring]; sector++ {
			if oculusSectorCarriesPayload(ring, sector) {
				total++
			}
		}
	}
	return total
}

func oculusRSN() int {
	return oculusPayloadBits() / 8
}

func oculusRSNSym() int {
	return oculusRSN() - oculusRSK
}
