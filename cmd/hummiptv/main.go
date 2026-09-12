package main

import (
	"log"

	nanotube "github.com/nanotube/nanotube-web"
	"github.com/nanotube/nanotube-web/internal/launcher"
	"github.com/nanotube/nanotube-web/internal/product"
)

func main() {
	if err := launcher.Run(product.HummIPTV, nanotube.FrontendAssets); err != nil {
		log.Fatal(err)
	}
}
