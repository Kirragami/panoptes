package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Kirragami/panoptes/eye/cli"
	"github.com/Kirragami/panoptes/eye/visions"
	"github.com/Kirragami/panoptes/eye/visions/dockerhealth"
)

func main() {
	if handled, err := cli.Handle(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintf(os.Stderr, "eye: %v\n", err)
			os.Exit(1)
		}
		return
	}

	iris, err := openIris()
	if err != nil {
		log.Fatalf("Eye could not open its Iris: %v", err)
	}

	eyeID, err := loadOrCreateEyeID(iris.StateDir)
	if err != nil {
		log.Fatalf("Eye failed to establish its identity: %v", err)
	}

	brand, err := recallBrand(iris.StateDir)
	if err != nil {
		log.Fatalf("Eye failed to recall its Brand: %v", err)
	}

	iris.Brand = brand

	epithet, err := resolveEpithet(iris.StateDir)
	if err != nil {
		log.Fatalf("Eye failed to take its Epithet: %v", err)
	}

	iris.Epithet = epithet

	log.Printf(
		"Eye identity: %s (%s); Panopticon: %s",
		epithet,
		eyeID,
		iris.PanopticonEndpoint,
	)

	// new visions will need to be added here, for later version aight?
	registry, err := visions.NewRegistry(
		dockerhealth.New(""),
	)
	if err != nil {
		log.Fatalf("Eye failed to acquire its Visions: %v", err)
	}

	maintainVigil(iris, eyeID, registry)
}
