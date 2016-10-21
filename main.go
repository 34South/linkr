package main

import (
	"log"
	"github.com/34South/envr"
)

var MongoDB *MongoConnection
var env *envr.Envr

func init() {

	env = envr.New("linkr-env", []string{
		"MONGO_URL",
		"MONGO_DB",
		"MONGO_LINKS_COLLECTION",
		"MONGO_STATS_COLLECTION",
	}).Passive().Fatal()
}

func main() {

	// Check the environment
	if env.Ready {
		log.Println("Environment looks good...")
	}
	// Create a connection to MongoDB
	MongoDB = NewMongoConnection()
	// Fire up the router
	Start()
}