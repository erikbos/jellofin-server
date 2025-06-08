# Jellofin Server

This is the Jellofin backend server. It support serving contents to clients using two different content APIs.

## Jellyfin API

This server supports a subset of the [Jellyfin API](https://api.jellyfin.org/). Most of the collection and media library is supported, enough to serve contents to [Infuse](https://firecore.com/infuse) and [Streamyfin](https://streamyfin.app/). Transcoding is not supported.

## Notflix API

- HTTP server for data (movies, images, etc) at `/data/<source-id>/path/...`
- Supports on the fly image resizing (?w=num&h=num&q=num) with a local image cache.
- JSON/REST API server at /api/
  - libraries (movies, tvshows, ...)
  - user data (auth, favorites, seen, ...)

### Notflix API definition

#### Collections

Encoding:

- request: parameters such as `:name` must be encoded using encodeURIComponent()
- reply: `uri` and `path` attributes are already encoded and must not be uri-encoded again in the request URL.

```
GET /api/collections
[ { ... lib1 ... },  { ... lib2 ... }, ... ]
```

```
GET /api/collection/:collectionname
{
  id: 3,
  name "library1",
  type: "movies",
  baseuri: "/data/1"
}
```

#### Items in a collection

Listing all items will get summary objects. For example a list of tv shows
will not include season and episode information for individual shows.

```
GET /api/collection/:collectionname/items
[
  {
    name: "aliens (1996)",
    baseurl: "/data/3",
    path: "aliens%20(1996)",
    ...
  },
]
```

Listing a single item will include details.

```
GET /api/collection/:collectionname/item/:itemname
{
  name: "aliens (1996)",
  path: "alien%20(1996)",
  ...
}
```

#### Data

The source of a collection will usually be one directory on the filesystem
of the server. A collection can have multiple sources though, so it can have
more than one directory, or even remote locations.

Each source of a collection is mapped to `/data/:source`. That's why the
baseuri is included in each item, since there can be multiple baseuris
in one collection.

## Acknowledgements

- [https://github.com/miquels/notflix-server](https://github.com/miquels/notflix-server) for original code this project is based upon.
- [https://jellyfin.org](jellyfin.org) for an awesome Mediaserver initiative.
