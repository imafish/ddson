# Distributed Download for Slow Office Network

This tool is meant to make download faster in an office with slow network speed.

It (initially) has 2 components, a server and a client.
The client send download request to the server and the server will try to download the file then send it to the client.

## How it works

It is designed much like p2p downloaders such as bit torrent. But it has a centralized server that caches all downloaded files.
It assumes that the LAN is fast but WAN is slow.
It caches downloaded files on the server, so other clients can get it very fast.
When the client initiate a download request to the server, the server distributes the download tasks to all registered clients. When download is finished, server sends the complete file to the client who started the download, and caches the file.

## Client Usage (TODO)

1. start
2. request download
3. stop

## How To Build

### install protoc compiler

Download from [protoc_compiler_github](https://github.com/protocolbuffers/protobuf/releases) and install to PATH

### install go compiler for protobuf and gRPC

``` bash
sudo go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
sudo go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## Build (manually)

1. compile .proto files:

``` bash
protoc --go_out=. --go-grpc_out=. api/proto/file-watcher-service.proto
```

2. build server:

``` bash
cd cmd/server
# build for macos
go --GOOS=darwin --GOARCH=arm64 build
# build for linux
go --GOOS=linux --GOARCH=amd64 build
```

3. build client

``` bash
cd cmd/client
go build
```
