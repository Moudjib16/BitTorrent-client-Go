# BitTorrent-client-Go

A small BitTorrent client written in Go. It downloads a `.torrent` file from
the command line and nothing else.

## What it does

- Parses single file and multi file `.torrent` files
- Announces to HTTP trackers and gets a list of peers
- Talks the BitTorrent peer protocol: handshake, bitfield, requests and pieces
- Downloads pieces in parallel and checks every piece against its SHA-1 hash
- Prints the percentage, speed, ETA and number of peers while it downloads
- Writes the data to disk while it downloads, so it never holds the whole
  torrent in memory

## Installing

Download a ready-made binary for Windows, Linux or macOS from the
[releases page](https://github.com/Moudjib16/BitTorrent-client-Go/releases),
or install it with Go:

```sh
go install github.com/Moudjib16/BitTorrent-client-Go@latest
```

`go install` names the program `BitTorrent-client-Go`, after the repository.

## Building and using

```sh
go build -o bittorrent-client-go .

./bittorrent-client-go ubuntu.torrent
./bittorrent-client-go -dir /downloads ubuntu.torrent
```

The torrent is downloaded into the directory given with `-dir` (the current
directory by default). Everything ends up in a folder named after the torrent.

## How the code is laid out

| Package | What it does |
| --- | --- |
| `torrentfile` | Parses `.torrent` files and writes the data to disk |
| `tracker` | Asks HTTP trackers for peers |
| `p2p` | The peer to peer protocol: handshake, messages, piece downloads |

## What it does not do

This is a minimal client, so a few things are missing:

- UDP trackers (BEP 15) are not supported
- No DHT, so peers that are only reachable through it are not found
- No magnet links
- It never seeds, it only downloads
- No resume support and no encryption

## Tests

```sh
go test ./...
```

The tests use a small fake peer and a fake tracker, so no network access is
needed.

## License

MIT, see [LICENSE](LICENSE).
