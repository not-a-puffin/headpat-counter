package main

import (
	"fmt"
	"headpat-counter/internal/server"
	"log"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	srv := server.New(port)

	fmt.Println("Server listening on port", port)
	log.Fatal(srv.ListenAndServe())
}
