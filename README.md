# linkr
Short link re-director written in Go

Started with this: https://github.com/minaandrawos/GoURLLinkShortnerAPI (thanks, Mina)

This is a simple API / service to enable short-link redirection. 

The short links are stored in a MongoDB collection, thus:

```javascript
{
	"_id" : ObjectId("5809363c73929f0003a3affb"),
	"createdAt" : ISODate("2016-10-20T21:25:16.898Z"),
	"updatedAt" : ISODate("2016-10-20T21:25:16.898Z"),
	"shortUrl" : "r2199",
	"longUrl" : "https://webcast.gigtv.com.au/Mediasite/Play/fa847e0ffef84d46a935bfad0bc5bd441d",
	"title" : "Does macrophage phenotype matter?",
	"clicks" : 3,
	"lastStatusCode" : 200
}
```

... and stores stats, thus:

```javascript
{
	"_id" : ObjectId("5808485073929f0003a3ab78"),
	"linkId" : ObjectId("580842e29257774f4174dd8f"),
	"createdAt" : ISODate("2016-10-20T04:30:08.236Z"),
	"referrer" : "https://github.com/34South/linkr/",
	"agent" : "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/53.0.2785.116 Safari/537.36",
	"statusCode" : 503
}
```

...which is handy for finding broken links.

It also checks that the target URL is functional before redirecting. This way you can opt to show your own error pages. 

Speaking of which, there are 2:

* error_l.html - served up when the remote target is bung
* error_s.html - served up when the short link / path cannot be found in the database

It requires the following environment vars:

```ini
MONGO_URL=mongodb://user:pass@host.com:[port]/dbname
MONGO_DB=dbName
MONGO_LINKS_COLLECTION=LinksCollectionName
MONGO_STATS_COLLECTION=LinkStatsCollectionName
```

Finally, once deployed a GET request to http://host.com/shortpath will do the do:

* Increment the `clicks` field
* Record a stats document
* Redirect (302) to th target, or server up an error page


### Deploy




Todo:
* Make error pages configurable via environment vars
* Add auth option











