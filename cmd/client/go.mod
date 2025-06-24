module ddson_client

go 1.24.4

replace internal/pb => ../../internal/pb

replace internal/version => ../../internal/version

replace internal/httputil => ../../internal/httputil

replace internal/common => ../../internal/common

replace internal/logging => ../../internal/logging

replace internal/progressbar => ../../internal/progressbar

require (
	golang.org/x/term v0.32.0
	google.golang.org/grpc v1.72.1
	internal/common v0.0.0
	internal/httputil v0.0.0
	internal/logging v0.0.0-00010101000000-000000000000
	internal/pb v0.0.0-00010101000000-000000000000
	internal/progressbar v0.0.0
	internal/version v0.0.0-00010101000000-000000000000
)

require (
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
