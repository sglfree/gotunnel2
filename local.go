package main

import (
  "log"
  "fmt"
  "./socks"
)

var defaultConfig = map[string]string{
  "local": "localhost:23456",
}

var globalConfig = loadConfig(defaultConfig)

func main() {
  socksServer, err := socks.New(globalConfig["local"])
  if err != nil {
    log.Fatal(err)
  }
  fmt.Printf("socks server listening on %s\n", globalConfig["local"])
  for {
    socksClient := (<-socksServer.Clients.Out).(*socks.Client)
    fmt.Printf("%v\n", socksClient)
  }
}
