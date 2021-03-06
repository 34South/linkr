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
	ID             bson.ObjectId `json:"_id,omitempty" bson:"_id"`
	CreatedAt      time.Time     `json:"createdAt" bson:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt" bson:"updatedAt"`
	ShortUrl       string        `json:"shortUrl" bson:"shortUrl"`
	LongUrl        string        `json:"longUrl" bson:"longUrl"`
	Title          string        `json:"title" bson:"title"`
	Clicks         int           `json:"clicks" bson:"clicks"`
	LastStatusCode int           `json:"lastStatusCode" bson:"lastStatusCode"`
	Active         bool          `json:"active"`
}

type LinkStatsDoc struct {
	ID         bson.ObjectId `json:"_id,omitempty" bson:"_id"`
	LinkID     bson.ObjectId `json:"linkId" bson:"linkId"`
	CreatedAt  time.Time     `json:"createdAt" bson:"createdAt"`
	Referrer   string        `json:"referrer" bson:"referrer"`
	Agent      string        `json:"agent" bson:"agent"`
	StatusCode int           `json:"statusCode" bson:"statusCode"`
}

type ResourcesDoc struct {
	ID          bson.ObjectId `json:"_id,omitempty" bson:"_id"`
	CreatedAt   time.Time     `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt" bson:"updatedAt"`
	PubDate                   `json:"pubDate" bson:"pubDate"`
	ShortUrl    string        `json:"shortUrl" bson:"shortUrl"`
	LongUrl     string        `json:"resourceUrl" bson:"resourceUrl"`
	Name        string        `json:"name" bson:"name"`
	Description string        `json:"description" bson:"description"`
	Active      bool          `json:"active" bson:"active"`
	Attributes                `json:"attributes" bson:"attributes"`
}

type PubDate struct {
	Date time.Time `json:"date" bson:"date"`
}

type Attributes struct {
	Source     string `json:"source" bson:"source"`
	SourceID   string `json:"sourceId" bson:"sourceId"`
	SourceName string `json:"sourceName" bson:"sourceName"`
	SourceNameAbbrev string `json:"sourceNameAbbrev" bson:"sourceNameAbbrev"`
	SourcePubDate  string `json:"sourcePubDate" bson:"sourcePubDate"`
	SourceVolume  string `json:"sourceVolume" bson:"sourceVolume"`
	SourceIssue  string `json:"sourceIssue" bson:"sourceIssue"`
	SourcePages  string `json:"sourcePages" bson:"sourcePages"`
}

type MongoConnection struct {
	Session      *mgo.Session
	URL          string
	DB           string
	LinksCol     string
	ResourcesCol string
	StatsCol     string
}

func NewMongoConnection() *MongoConnection {

	c := new(MongoConnection)
	c.URL = os.Getenv("MONGO_URL")
	c.DB = os.Getenv("MONGO_DB")
	c.LinksCol = os.Getenv("MONGO_LINKS_COLLECTION")
	c.ResourcesCol = os.Getenv("MONGO_RESOURCES_COLLECTION")
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

	ResourcesCollection := c.Session.DB(c.DB).C(c.ResourcesCol)
	if ResourcesCollection == nil {
		err = errors.New("Could not create or attach to collection: " + c.ResourcesCol)
	} else {
		log.Printf("Found collection %s\n", c.ResourcesCol)
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

func (c *MongoConnection) sessionLinksCollection() (session *mgo.Session, urlCollection *mgo.Collection, err error) {

	if c.Session != nil {
		session = c.Session.Copy()
		urlCollection = session.DB(c.DB).C(c.LinksCol)
	} else {
		err = errors.New("No original session found")
	}

	return
}

func (c *MongoConnection) sessionResourcesCollection() (session *mgo.Session, urlCollection *mgo.Collection, err error) {

	if c.Session != nil {
		session = c.Session.Copy()
		urlCollection = session.DB(c.DB).C(c.ResourcesCol)
	} else {
		err = errors.New("No original session found")
	}

	return
}

func (c *MongoConnection) sessionStatsCollection() (session *mgo.Session, urlCollection *mgo.Collection, err error) {

	if c.Session != nil {
		session = c.Session.Copy()
		urlCollection = session.DB(c.DB).C(c.StatsCol)
	} else {
		err = errors.New("No original session found")
	}

	return
}

func (c *MongoConnection) FindShortUrl(longurl string) (sUrl string, err error) {

	//create an empty document struct
	result := LinkDoc{}
	//get a copy of the original session and a collection
	session, urlCollection, err := c.sessionLinksCollection()
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

func (c *MongoConnection) FindLink(shortUrl string) (LinkDoc, error) {

	l := LinkDoc{}

	//get a copy of the original session and a collection
	session, lc, err := c.sessionLinksCollection()
	if err != nil {
		return l, err
	}
	defer session.Close()

	//Find the link doc
	err = lc.Find(bson.M{"shortUrl": shortUrl}).One(&l)
	if err != nil {
		return l, err
	}

	return l, nil
}

func (c *MongoConnection) AddLink(ld LinkDoc) error {

	//get a copy of the session
	session, lc, err := c.sessionLinksCollection()
	if err != nil {
		return err
	}
	defer session.Close()

	// Use shortlink as a selector for an upsert
	s := bson.M{"shortUrl": ld.ShortUrl}

	//insert a document with the provided function arguments
	err = lc.UpdateId(s, ld)
	if err != nil {
		// No need as we are upserting
		////check if the error is due to duplicate shorturl
		//if mgo.IsDup(err) {
		//	err = errors.New("Duplicate value for shortUrl")
		//}
		return err
	}

	return nil
}

func (c *MongoConnection) IncrementClicks(shortUrl string) error {

	//get a copy of the original session and a collection
	session, urlCollection, err := c.sessionLinksCollection()
	if err != nil {
		return err
	}
	defer session.Close()

	err = urlCollection.Update(bson.M{"shortUrl": shortUrl}, bson.M{"$inc": bson.M{"clicks": 1}})
	if err != nil {
		return err
	}
	return nil
}

func (c *MongoConnection) UpdateStatusCode(shortUrl string, statusCode int) error {

	session, urlCollection, err := c.sessionLinksCollection()
	if err != nil {
		return err
	}
	defer session.Close()

	err = urlCollection.Update(bson.M{"shortUrl": shortUrl}, bson.M{"$set": bson.M{"lastStatusCode": statusCode}})
	if err != nil {
		return err
	}
	return nil
}

func (c *MongoConnection) RecordStats(s LinkStatsDoc) error {

	//get a copy of the original session and a collection
	session, collection, err := c.sessionStatsCollection()
	if err != nil {
		return err
	}
	defer session.Close()

	err = collection.Insert(s)
	if err != nil {
		return err
	}
	return nil
}

func (c *MongoConnection) Popular(n int) ([]LinkDoc, error) {

	var r []LinkDoc

	//get a copy of the original session and a collection
	session, collection, err := c.sessionLinksCollection()
	if err != nil {
		return r, err
	}
	defer session.Close()

	err = collection.Find(bson.M{"clicks": bson.M{"$gt": 0}}).Limit(n).Sort("-clicks").All(&r)
	if err != nil {
		return r, err
	}

	return r, nil
}

// Latest queries the Resources collection and orders by pubDate.date desc
func (c *MongoConnection) Latest(n int) ([]ResourcesDoc, error) {

	var r []ResourcesDoc

	//get a copy of the original session and a collection
	session, collection, err := c.sessionResourcesCollection()
	if err != nil {
		return r, err
	}
	defer session.Close()

	err = collection.Find(bson.M{"active": true}).Limit(n).Sort("-pubDate.date").All(&r)
	if err != nil {
		return r, err
	}

	return r, nil
}

func (c *MongoConnection) Broken() ([]LinkDoc, error) {

	var r []LinkDoc

	//get a copy of the original session and a collection
	session, collection, err := c.sessionLinksCollection()
	if err != nil {
		return r, err
	}
	defer session.Close()

	err = collection.Find(bson.M{"lastStatusCode": bson.M{"$exists": true, "$nin": []int{200, 0}}}).Sort("-clicks").All(&r)
	if err != nil {
		return r, err
	}

	return r, nil
}
