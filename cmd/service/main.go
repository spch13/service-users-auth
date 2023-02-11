package main

import (
	"github.com/spch13/service-users-auth/internal/app"
	"log"
)

func main() {
	app, err := app.New()
	if err != nil {
		log.Fatalln(err)
	}

	if err := app.Run(); err != nil {
		log.Fatalln(err)
	}
}
