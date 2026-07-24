package main

import (
	"log"
)

func main() {
	iris, err := openIris()
	if err != nil {
		log.Fatalf("Eye could not open its Iris: %v", err)
	}

	eyeID, err := loadOrCreateEyeID("./state")
	if err != nil {
		log.Fatalf("Eye failed to establish its identity: %v", err)
	}

	log.Printf(
		"Eye identity: %s; Panopticon: %s",
		eyeID,
		iris.PanopticonEndpoint,
	)
}
