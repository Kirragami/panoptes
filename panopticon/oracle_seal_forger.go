package main

import (
	"encoding/base64"
	"html/template"
)

func forgeOracleSealSigil(oracleSeal string) (template.URL, error) {
	svg, err := renderOculusSVG(oracleSeal)
	if err != nil {
		return "", err
	}

	return template.URL(
		"data:image/svg+xml;base64," +
			base64.StdEncoding.EncodeToString([]byte(svg)),
	), nil
}
