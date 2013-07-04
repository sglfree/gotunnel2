all:
	go build local.go config.go session.go
	go build server.go config.go session.go
