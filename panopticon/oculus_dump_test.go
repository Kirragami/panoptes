package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOculusDumpRoundtripArtifacts(t *testing.T) {
	seal := "6a8ac4541993b83b7f5c097c65bea7734a588ffa550a92686dd9306f0c495bf9"
	bits, err := encodeOculusPayload(seal)
	if err != nil {
		t.Fatal(err)
	}
	svg, err := renderOculusSVG(seal)
	if err != nil {
		t.Fatal(err)
	}

	type cell struct {
		Ring   int `json:"ring"`
		Sector int `json:"sector"`
		Bit    int `json:"bit"`
		Value  int `json:"value"`
	}
	cells := make([]cell, 0, len(bits))
	bit := 0
	for ring := 0; ring < oculusRingCount; ring++ {
		for sector := 0; sector < oculusSectorsPerRing[ring]; sector++ {
			if !oculusSectorCarriesPayload(ring, sector) {
				continue
			}
			cells = append(cells, cell{
				Ring:   ring,
				Sector: sector,
				Bit:    bit,
				Value:  int(bits[bit]),
			})
			bit++
		}
	}

	dir := filepath.Clean("../")
	_ = os.WriteFile(filepath.Join(dir, "tmp_oculus_roundtrip.svg"), []byte(svg), 0644)
	bitInts := make([]int, len(bits))
	for i, b := range bits {
		bitInts[i] = int(b)
	}
	payload, _ := json.MarshalIndent(map[string]any{
		"seal":        seal,
		"payloadBits": oculusPayloadBits(),
		"version":     oculusVersion,
		"bits":        bitInts,
		"cells":       cells,
		"finders":     oculusInCircleFinders(),
		"corners":     oculusCorners(),
		"center":      []float64{oculusCenterX, oculusCenterY},
		"viewSize":    oculusViewSize,
	}, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "tmp_oculus_layout.json"), payload, 0644)
}
