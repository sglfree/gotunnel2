all:
	go build local.go common.go
	go build server.go common.go
