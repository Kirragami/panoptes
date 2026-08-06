package main

import (
	"log"
)

func main() {
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

	log.Printf(
		"Eye identity: %s; Panopticon: %s",
		eyeID,
		iris.PanopticonEndpoint,
	)

	maintainVigil(iris, eyeID)
}
