package main

import (
  "log"
  "fmt"
  "./socks"
  cr "./conn_reader"
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
  connReader := cr.New()
  go func() {
    for {
      socksClient := (<-socksServer.Clients.Out).(*socks.Client)
      connReader.Add(socksClient.Conn)
    }
  }()
  for {
    msg := (<-connReader.Messages.Out).(cr.Message)
    fmt.Printf("%v\n", msg)
  }
}
