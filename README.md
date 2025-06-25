# Jellofin Server

This is the Jellofin backend server. It support serving contents to clients using two different server APIs:

1. Jellyfin API
2. Notflix API

## Jellyfin API

This server supports a subset of the [Jellyfin API](https://api.jellyfin.org/). Most (all?) of the collection and media library endpoints are implemented. This server can be used to serve contents to [Infuse](https://firecore.com/infuse) and [Streamyfin](https://streamyfin.app/). Transcoding of contents is not supported.

## Notflix API

- HTTP server for data (movies, images, etc) at `/data/<source-id>/path/...`
- Supports on the fly image resizing (?w=num&h=num&q=num) with a local image cache.
- JSON/REST API server at /api/
  - libraries (movies, tvshows, ...)
  - user data (auth, favorites, seen, ...)

## Installation

1. `go build` will compile `jellofin-server`
2. copy `jellofin-server.example.cfg` to `jellofin-server.cfg` and edit collection configuration details
4. start `jellofin-server`

## Collection format

Every collection has a type, either ``movies` or `tvshows`.

For type `movies` the expected directory format and file naming is:

```
movies/
├── Movie 1 (1984)/
│   └── movie.mp4
└── Movie 2 (2001)/
    └── movie.mp4
```

For type `tvshows` the expected directory format and file naming is:

```
tvshows/
└── ShowName/
    ├── Season 1/
    │   ├── S01E01 - EpisodeName.mp4
    │   └── S01E02 - EpisodeName.mp4
    └─── Season 2/
        ├── S02E01 - EpisodeName.mp4
        └── S02E02 - EpisodeName.mp4
```

### Data

The source of a collection will usually be one directory on the filesystem
of the server. A collection can have multiple sources though, so it can have
more than one directory, or even remote locations.

Each source of a collection is mapped to `/data/:source`. That's why the
baseuri is included in each item, since there can be multiple baseuris
in one collection.

## Acknowledgements

- [https://github.com/miquels/notflix-server](https://github.com/miquels/notflix-server) for original code this project is based upon.
- [https://jellyfin.org](jellyfin.org) for an awesome Mediaserver initiative.
