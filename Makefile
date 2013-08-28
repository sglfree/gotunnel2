all:
	go build local.go common.go
	go build server.go common.go

win:
	GOOS=windows GOARCH=386 go build -o server-32.exe server.go common.go
	GOOS=windows GOARCH=386 go build -o local-32.exe local.go common.go
	GOOS=windows GOARCH=amd64 go build -o server-64.exe server.go common.go
	GOOS=windows GOARCH=amd64 go build -o local-64.exe local.go common.go

race:
	go build -race local.go common.go
	go build -race server.go common.go
