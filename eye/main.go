package main

import (
	"log"
)

func main() {
	eyeID, err := loadOrCreateEyeID("./state")
	if err != nil {
		log.Fatalf("Eye failed to establish its identity: %v", err)
	}

	log.Printf("Eye identity: %s", eyeID)
}