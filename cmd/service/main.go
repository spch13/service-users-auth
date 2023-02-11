package main

import (
	"log"
	"service-users-auth/internal/app"
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
