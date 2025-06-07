package main

//go:generate go run src/install/updateversion.go
//go:generate git commit -am "bump $VERSION"
//go:generate git tag -af v$VERSION -m "v$VERSION"

import (
	"fmt"
	"log"
	"trustdrop/gui"
)

func main() {
	app, err := gui.NewTrustDropApp()
	if err != nil {
		log.Fatalf("Failed to initialize TrustDrop: %v", err)
	}
	
	fmt.Println("TrustDrop - Secure Blockchain File Transfer")
	fmt.Println("==========================================")
	fmt.Println("Starting application...")
	
	app.Run()
}