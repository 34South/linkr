package main

import (
	"github.com/34South/envr"
	"log"
)

var MongoDB *MongoConnection
var env *envr.Envr

func init() {

	env = envr.New("linkr-env", []string{
		"MONGO_URL",
		"MONGO_DB",
		"MONGO_LINKS_COLLECTION",
		"MONGO_STATS_COLLECTION",
		"LINKR_BASE_URL",
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
