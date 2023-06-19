package main

import "net/http"

func main() {
	mux := http.NewServeMux()
	mux.Handle("/v1/users", &userHandler{})
	mux.Handle("/v1/organizations", &organizationHandler{})
	mux.Handle("/v1/deployments", &deploymentHandler{})
	s := &http.Server{
		Addr:    "localhost:3001",
		Handler: mux,
	}

	err := s.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
