
# Notflix Server

This is the Notflix backend server. It does a couple of things:

- HTTP server for data (movies, images, etc) at /\_data/<source-id>/path/...
- Supports image resizing (?w=num&h=num&q=num) with a local cache
- JSON/REST API server at /\_api/
  - libraries (movies, tvshows, ...)
  - server configuration
  - user data (auth, favorites, seen, ...)
- HTTP server for the webapp at /

## Collections

Encoding:
- request: parameters such as :name must be encoded using encodeURIComponent()
- reply: 'uri' and 'path' attributes are already encoded and must not
  be uri-encoded again in the request URL.


```
GET /\_api/collections
[ { ... lib1 ... },  { ... lib2 ... }, ... ]
```

```
GET /\_api/collection/:collectionname
{
  id: 3,
  name "library1",
  type: "movies",
  baseuri: "/\_data/1"
}
```

## Items in a collection

Listing all items will get summary objects. For example a list of tv shows
will not include season and episode information for individual shows.

```
GET /\_api/collection/:collectionname/items
[
  {
    name: "aliens (1996)",
    baseurl: "/_data/3",
    path: "aliens%20(1996)",
    ...
  },
]
```

Listing a single item will include details.

```
GET /\_api/collection/:collectionname/item/:itemname
{
  name: "aliens (1996)",
  path: "alien%20(1996)",
  ...
}
```

## Data

The source of a collection will usually be one directory on the filesystem
of the server. A collection can have multiple sources though, so it can have
more than one directory, or even remote locations.

Each source of a collection is mapped to /\_data/:source. That's why the
baseuri is included in each item, since there can be multiple baseuris
in one collection.

## Development

ImageMagick v6 is required:

```shell
# We still require Imagemagick v6, downgrade in case you've got v4:
brew unlink imagemagick
brew install imagemagick@6 && brew link imagemagick@6 --force
```
