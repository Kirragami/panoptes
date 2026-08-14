package main

import (
	"testing"
)

func TestOculusEncodeRoundTrip(t *testing.T) {
	seal := "6a8ac4541993b83b7f5c097c65bea7734a588ffa550a92686dd9306f0c495bf9"

	bits, err := encodeOculusPayload(seal)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(bits) != oculusPayloadBits() {
		t.Fatalf("bit length %d want %d", len(bits), oculusPayloadBits())
	}

	decoded, err := decodeOculusPayload(bits)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded != seal {
		t.Fatalf("decoded %s want %s", decoded, seal)
	}

	broken := append([]byte(nil), bits...)
	broken[50] ^= 1
	if _, err := decodeOculusPayload(broken); err == nil {
		t.Fatal("expected decode failure after bit flip")
	}
}

func TestOculusRenderSVG(t *testing.T) {
	seal := "73ad84274c83ae01c5a40cef67b65a59636784a08c70c8f68782b48cf872b971"
	svg, err := renderOculusSVG(seal)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(svg) < 1000 {
		t.Fatalf("svg too short: %d", len(svg))
	}
	if svg[:4] != "<svg" {
		t.Fatalf("missing svg open tag")
	}

	url, err := forgeOracleSealSigil(seal)
	if err != nil {
		t.Fatalf("forge: %v", err)
	}
	if len(url) < 100 || string(url)[:26] != "data:image/svg+xml;base64," {
		t.Fatalf("unexpected data url prefix")
	}
}

func TestOculusGeometryBudget(t *testing.T) {
	if len(oculusSectorsPerRing) != oculusRingCount {
		t.Fatalf("ring count mismatch")
	}
	if oculusPayloadBits()%8 != 0 {
		t.Fatalf("payload bits not byte aligned")
	}
	if oculusRSN() != oculusRSK+oculusRSNSym() {
		t.Fatalf("rs sizes inconsistent")
	}
	if oculusVersion != 7 {
		t.Fatalf("version %d want 7", oculusVersion)
	}
	if oculusOuterR != 520 {
		t.Fatalf("outer radius %v want 520", oculusOuterR)
	}
	if oculusFinderModule != 18 {
		t.Fatalf("finder module %v want 18", oculusFinderModule)
	}
	if oculusRingWidth != 54 {
		t.Fatalf("ring width %v want 54", oculusRingWidth)
	}
	// Phase locks only; finders live on the outer black rim.
	if oculusPayloadBits() != 384 {
		t.Fatalf("payload bits %d want 384", oculusPayloadBits())
	}
	if oculusRSNSym() < 8 {
		t.Fatalf("Reed-Solomon redundancy too small: %d", oculusRSNSym())
	}
	if len(oculusInCircleFinders()) != 4 {
		t.Fatalf("expected 4 rim finders")
	}
	orbit := oculusFinderOrbitR()
	if orbit <= oculusOuterR+42 {
		t.Fatalf("finder orbit %v must clear outer timing rim", orbit)
	}
	if !oculusTimingSectorIsLight(0) || !oculusTimingSectorIsLight(1) {
		t.Fatalf("timing zero mark must light sectors 0 and 1")
	}
	if oculusTimingSectorIsLight(2) {
		t.Fatalf("timing sector 2 should be dark after zero mark")
	}
}
