# Jellofin Server

This is the Jellofin Media Server server. It support serving contents to clients using two different server APIs:

1. Jellyfin API
2. Notflix API

## Jellyfin API

This server supports a subset of the [Jellyfin API](https://api.jellyfin.org/). Most (all?) of the collection and media library endpoints are implemented. All contents is served as is. Transcoding of contents is not supported and is not foreseen to be added.

### Tested clients

The following clients can connect to Jellofin:

| Client                                           | Status | Notes                     |
| :----------------------------------------------: | :----: | :-----------------------: |
| [Infuse](https://firecore.com/infuse)            | ✅      | Full player functionality |
| [Senplayer](https://apps.apple.com/ca/app/senplayer-video-media-player/id6443975850) | ✅      | Full player functionality |
| [Streamyfin](https://streamyfin.app/)            | ✅      | Full player functionality |
| [VidHub](https://okaapps.com/product/1659622164) | ✅      | Full player functionality |

## Notflix API

- HTTP server for data (movies, images, etc) at `/data/<source-id>/path/...`
- Supports on the fly image resizing (?w=num&h=num&q=num) with a local image cache.
- JSON/REST API server at /api/
  - libraries (movies, tvshows, ...)
  - user data (auth, favorites, seen, ...)

## Installation

1. run `go build` to compile `jellofin-server`
2. copy `jellofin-server.example.yaml` to `jellofin-server.yaml` and edit collection configuration details
3. run `./jellofin-server` to start the server

## Configuration File

The server uses a YAML configuration file (default: `jellofin-server.yaml`). Below are all supported configuration values and their descriptions:

## Top-level keys

| Key           | Type    | Description                                                                 |
|---------------|---------|-----------------------------------------------------------------------------|
| `listen`      | object  | Network settings for the server.                                            |
| `appdir`      | string  | Path to the directory containing the web UI/static files.                   |
| `cachedir`    | string  | Path to the directory for image cache storage.                              |
| `dbdir`       | string  | Legacy: directory where a DB file may be stored (kept for backwards compat).|
| `database`    | object  | Database backend configuration.                                             |
| `logfile`     | string  | Log output: file path, `stdout`, `syslog`, or `none`.                       |
| `collections` | array   | List of media collections served by the server.                             |
| `jellyfin`    | object  | Jellyfin API-specific settings.                                             |

---

### `listen` section

| Key       | Type   | Description                                  |
|-----------|--------|----------------------------------------------|
| `address` | string | Address to bind the server (e.g., `0.0.0.0`).|
| `port`    | int    | Port to listen on (e.g., `8096`).            |
| `tlscert` | string | Path to TLS certificate file (optional).     |
| `tlskey`  | string | Path to TLS private key file (optional).     |
| `ipacl`   | string | IP allow list, If set only matching CIDRs may access the server (e.g. `127.0.0.1/32, 192.168.1.0/24`). |

---

### `database` section

The current default database driver is for SQLite. Configure the sqlite backend *under* the `database` key.

| Key               | Type   | Description                                                                   |
| ----------------- | ------ | ----------------------------------------------------------------------------- |
| `sqlite`          | object | SQLite-specific configuration.                                                |
| `sqlite.filename` | string | Full path to the sqlite database file (e.g. `/var/lib/jellofin/jellofin.db`). |

---

### `collections` section

Each entry defines a media collection:

| Key         | Type   | Description                                                     |
| ----------- | ------ | --------------------------------------------------------------- |
| `id`        | string | Optional override for collection ID (expert use!).              |
| `name`      | string | Display name of the collection.                                 |
| `type`      | string | Type of collection: `movies`, `shows`.                          |
| `directory` | string | Filesystem path to the media files.                             |
| `baseurl`   | string | Base URL for accessing the collection (optional).               |
| `hlsserver` | string | URL of the HLS server for streaming (optional).                 |

---

### `jellyfin` section

| Key                  | Type    | Description                                                  |
| -------------------- | ------- | ------------------------------------------------------------ |
| `servername`         | string  | Name of the server as shown to clients.                      |
| `autoregister`       | boolean | If set to true, unknown users will be auto registered        |
| `imagequalityposter` | int     | Poster image quality (1-100, lower = smaller).               |
| `serverid`           | string  | Optional override for server ID (expert use!).               |

---

## Example configuration file

```yaml
listen:
  address: 0.0.0.0
  port: 8096

appdir: /srv/jellofin/ui
cachedir: /srv/jellofin/cache
logfile: stdout

database:
  sqlite:
    filename: /usr/local/jellofin/db/jellofin.db

collections:
  - id: movies
    name: Movies
    type: movies
    directory: /srv/media/movies
    baseurl: /media/movies
    hlsserver: http://localhost:6453/media/movies/
  - id: shows
    name: TV Shows
    type: shows
    directory: /srv/media/shows
    baseurl: /media/shows
    hlsserver: http://localhost:6453/media/shows/

jellyfin:
  servername: My media server
  autoregister: true
```

## Collection format

Every collection has a type, either `movies` or `tvshows`.

For type `movies` the expected directory format and file naming is:

```text
movies/
├── Movie 1 (1984)/
│   └── movie.mp4
└── Movie 2 (2001)/
    └── movie.mp4
```

For type `tvshows` the expected directory format and file naming is:

```text
tvshows/
└── ShowName/
    ├── Season 1/
    │   ├── S01E01 - EpisodeName.mp4
    │   └── S01E02 - EpisodeName.mp4
    └─── Season 2/
        ├── S02E01 - EpisodeName.mp4
        └── S02E02 - EpisodeName.mp4
```

Tvshows season number 0 are renamed to 'Specials' and have 99 as internal to force them to appear as "last" season.

### Unsupported folder layouts:

Nested folders are not supported:

```text
movies/
├── History/
│   └── Movie 1 (1984)/
│       └── movie.mp4
│   └── Movie 2 (2001)/
│       └── movie.mp4
```

_Workaround: specify each subfolder individually in configuration._

2. Mixed Movies & Shows

Unsupported:

```text
movies/
├── Movie 1 (1984)/
│   └── movie.mp4
└── ShowName/
│   ├── Season 1/
│   │   ├── S01E01 - EpisodeName.mp4
│   │   └── S01E02 - EpisodeName.mp4
└── Movie 2 (2001)/
    └── movie.mp4
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
