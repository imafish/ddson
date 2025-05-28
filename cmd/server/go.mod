module ddson_server

go 1.23.3

replace internal/pb => ../../internal/pb

replace internal/version => ../../internal/version

replace internal/httputil => ../../internal/httputil

replace internal/common => ../../internal/common

require (
	google.golang.org/grpc v1.72.1
	internal/common v0.0.0-00010101000000-000000000000
	internal/httputil v0.0.0
	internal/pb v0.0.0
	internal/version v0.0.0
)

require (
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)
