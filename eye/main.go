package main

import (
	"context"
	"log"
	"time"
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

	log.Printf(
		"Eye identity: %s; Panopticon: %s",
		eyeID,
		iris.PanopticonEndpoint,
	)

	bindContext, cancelBind := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelBind()

	if err := bindEye(bindContext, iris.PanopticonEndpoint, eyeID); err != nil {
		log.Fatalf("Eye could not Bind to Panopticon: %v", err)
	}

	log.Printf("Eye successfully bound to Panopticon: %s", eyeID)

	if err := keepVigil(
		context.Background(),
		iris.PanopticonEndpoint,
		eyeID,
	); err != nil {
		log.Fatalf("Eye lost its Vigil: %v", err)
	}
}
