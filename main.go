package main

//go:generate go run src/install/updateversion.go
//go:generate git commit -am "bump $VERSION"
//go:generate git tag -af v$VERSION -m "v$VERSION"

import (
	"trustdrop/gui"
)

func main() {
	app := gui.NewTrustDropApp()
	app.Run()
}
