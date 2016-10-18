package main

import (
	"errors"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"os"
	"time"
)

type LinkDoc struct {
	ID          bson.ObjectId `json:"_id,omitempty" bson:"_id"`
	CreatedAt   time.Time     `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt" bson:"updatedAt"`
	ShortUrl    string        `json:"shortUrl" bson:"shortUrl"`
	LongUrl     string        `json:"longUrl" bson:"longUrl"`
	Title       string        `json:"title" bson:"title"`
	Description string        `json:"description" bson:"description"`
	Clicks      int           `json:"clicks" bson:"clicks"`
}

type MongoConnection struct {
	Session  *mgo.Session
	URL      string
	DB       string
	LinksCol string
	StatsCol string
}

func NewMongoConnection() *MongoConnection {

	c := new(MongoConnection)
	c.URL = os.Getenv("MONGO_URL")
	c.DB = os.Getenv("MONGO_DB")
	c.LinksCol = os.Getenv("MONGO_LINKS_COLLECTION")
	c.StatsCol = os.Getenv("MONGO_STATS_COLLECTION")
	c.CreateConnection()

	return c
}

func (c *MongoConnection) CreateConnection() (err error) {

	log.Println("Connecting to Mongo server....")
	c.Session, err = mgo.Dial(c.URL)
	if err != nil {
		log.Fatalf("Error occured while creating mongodb connection: %s\n", err.Error())
	}

	log.Println("Connected to server!")
	LinksCollection := c.Session.DB(c.DB).C(c.LinksCol)
	if LinksCollection == nil {
		err = errors.New("Could not create or attach to collection: " + c.LinksCol)
	} else {
		log.Printf("Found collection %s\n", c.LinksCol)
	}

	StatsCollection := c.Session.DB(c.DB).C(c.StatsCol)
	if StatsCollection == nil {
		err = errors.New("Could not create or attach to collection: " + c.StatsCol)
	} else {
		log.Printf("Found collection %s\n", c.StatsCol)
	}

	//This will create a unique index to ensure that there won't be duplicate shorturls in the database.
	index := mgo.Index{
		Key:      []string{"$text:shortUrl"},
		Unique:   true,
		DropDups: true,
	}
	LinksCollection.EnsureIndex(index)

	return err
}

func (c *MongoConnection) CloseConnection() {
	if c.Session != nil {
		c.Session.Close()
	}
}

func (c *MongoConnection) getSessionAndCollection() (session *mgo.Session, urlCollection *mgo.Collection, err error) {

	if c.Session != nil {
		session = c.Session.Copy()
		urlCollection = session.DB(c.DB).C(c.LinksCol)
	} else {
		err = errors.New("No original session found")
	}

	return
}

func (c *MongoConnection) FindShortUrl(longurl string) (sUrl string, err error) {

	//create an empty document struct
	result := LinkDoc{}
	//get a copy of the original session and a collection
	session, urlCollection, err := c.getSessionAndCollection()
	if err != nil {
		return
	}
	defer session.Close()

	err = urlCollection.Find(bson.M{"longUrl": longurl}).One(&result)
	if err != nil {
		return
	}

	return result.ShortUrl, nil
}

func (c *MongoConnection) IncrementClicks(shortUrl string) error {

	//get a copy of the original session and a collection
	session, urlCollection, err := c.getSessionAndCollection()
	if err != nil {
		return err
	}
	defer session.Close()

	err = urlCollection.Update(bson.M{"shortUrl": shortUrl}, bson.M{"$inc": bson.M{"Clicks": 1}})
	if err != nil {
		return err
	}
	return nil
}

func (c *MongoConnection) FindLongUrl(shortUrl string) (lUrl string, err error) {

	//create an empty document struct
	result := LinkDoc{}
	//get a copy of the original session and a collection
	session, urlCollection, err := c.getSessionAndCollection()
	if err != nil {
		return
	}
	defer session.Close()
	//Find the shorturl that we need
	err = urlCollection.Find(bson.M{"shortUrl": shortUrl}).One(&result)
	if err != nil {
		return
	}
	return result.LongUrl, nil
}

func (c *MongoConnection) AddUrl(longUrl string, shortUrl string) (err error) {
	//get a copy of the session
	session, urlCollection, err := c.getSessionAndCollection()
	if err == nil {
		defer session.Close()
		//insert a document with the provided function arguments
		err = urlCollection.Insert(
			&LinkDoc{
				ID:        bson.NewObjectId(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				ShortUrl:  shortUrl,
				LongUrl:   longUrl,
			},
		)
		if err != nil {
			//check if the error is due to duplicate shorturl
			if mgo.IsDup(err) {
				err = errors.New("Duplicate value for shortUrl")
			}
		}
	}
	return
}
