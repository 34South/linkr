package main

import (
	"github.com/34South/envr"
	"html/template"
)

var MongoDB *MongoConnection
var tpl *template.Template

func init() {

	envr.New("linkr-env", []string{
		"MONGO_URL",
		"MONGO_DB",
		"MONGO_LINKS_COLLECTION",
		"MONGO_STATS_COLLECTION",
		"LINKR_BASE_URL",
	}).Auto()

	tpl = template.Must(template.ParseGlob("./templates/*.gohtml"))
}

func main() {

	// Create a connection to MongoDB
	MongoDB = NewMongoConnection()

	// Fire up the router
	Start()
}
