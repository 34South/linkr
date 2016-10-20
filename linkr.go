package main

import (
	"bufio"
	"log"
	"os"
	"strings"
	"github.com/mappcpd/envr"
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

	// Create a connection o MongoDB
	MongoDB = NewMongoConnection()
	// Fire up the router
	Start()
}

// initEnv initialises environment vars. For local development and testing we want
// to set these from a .env file. However we DO NOT want the .env file to be part of the repo
// so, in the case of Herou deployment, the env vars are set via the cli or web interface
// // and are then available to the application. So here we just test to see if we already have
// one of our vars, and if so, we can ssume we don't need to read .env, otherwise read .env.
func initEnv() {

	if len(os.Getenv("MONGO_URL")) < 1 {
		log.Println("Missing environment vars, will try to set from .env file...")
		envOK := dotEnv()
		if !envOK {
			log.Fatalln("Failed to initialise environment vars from .env file")
		}
	}
}

// dotEnv will initialise the environment vars from the local .env file.
func dotEnv() bool {

	// TODO define a struct for .env and check we have all the values we need
	log.Println("Initialising environment...")

	// Read in the .env file
	f, err := os.Open(".env")
	if err != nil {
		log.Fatalf("Error opening .env file: %s", err.Error())
		return false
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		v := strings.Split(scanner.Text(), "=")
		if len(v) == 2 {
			os.Setenv(v[0], v[1])
		}
	}
	// TODO check all required config values exist
	return true
}
