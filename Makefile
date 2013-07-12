all:
	go build local.go common.go
	go build server.go common.go

race:
	go build -race local.go common.go
	go build -race server.go common.go
