# Notflix API definition

## Collections

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

## Genres of a collection

```
GET /api/collection/:collectionname/genres'
{
  "Action": 2,
  "Adventure": 5,
  "Romance": 1
}
```

## Items in a collection

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