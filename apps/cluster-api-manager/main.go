package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/sharingio/pair/common"
	"github.com/sharingio/pair/kubernetes"
	"github.com/sharingio/pair/routes"
)

func handleWebserver() {
	// bring up the API
	port := common.GetAppPort()
	router := mux.NewRouter().StrictSlash(true)
	apiEndpointPrefix := "/api"

	err, clientset := kubernetes.Client()
	if err != nil {
		log.Panicln(err)
		return
	}

	err, kubernetesDynamicClientset := kubernetes.DynamicClient()
	if err != nil {
		log.Panicln(err)
		return
	}

	err, restConfig := kubernetes.RestClient()
	if err != nil {
		log.Panicln(err)
		return
	}

	for _, endpoint := range routes.GetEndpoints(apiEndpointPrefix, clientset, kubernetesDynamicClientset, restConfig) {
		router.HandleFunc(endpoint.EndpointPath, endpoint.HandlerFunc).Methods(endpoint.HttpMethods...)
	}

	router.HandleFunc(apiEndpointPrefix+"/{.*}", routes.APIUnknownEndpoint)
	router.HandleFunc(apiEndpointPrefix, routes.APIroot)
	router.Use(common.Logging)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowCredentials: true,
	})

	srv := &http.Server{
		Handler:      c.Handler(router),
		Addr:         port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Println("Listening on", port)
	log.Fatal(srv.ListenAndServe())
}

func main() {
	// initialise the app
	handleWebserver()
}
