module ddson_server

go 1.24.4

replace (
	internal/agents => ../../internal/agents
	internal/common => ../../internal/common
	internal/database => ../../internal/database
	internal/httputil => ../../internal/httputil
	internal/logging => ../../internal/logging
	internal/pb => ../../internal/pb
	internal/persistency => ../../internal/persistency
	internal/version => ../../internal/version
)

require (
	golang.org/x/term v0.32.0
	google.golang.org/grpc v1.73.0
	internal/agents v0.0.0
	internal/common v0.0.0
	internal/httputil v0.0.0
	internal/logging v0.0.0
	internal/pb v0.0.0
	internal/persistency v0.0.0
	internal/version v0.0.0
)

require (
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/mattn/go-sqlite3 v1.14.28 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	internal/database v0.0.0 // indirect
)
