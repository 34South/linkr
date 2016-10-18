package main

var MongoDB *MongoConnection

func main() {

	//Create a new API shortner API
	MongoDB = NewMongoConnection()

	Start()
}
