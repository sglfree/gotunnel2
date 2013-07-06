all:
	go build local.go config.go
	go build server.go config.go
