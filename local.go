package main

import (
  "fmt"
)

var defaultConfig = map[string]string{
  "local": "localhost:23456",
}

var globalConfig = loadConfig(defaultConfig)

func main() {
  fmt.Printf("%v\n", globalConfig)
}
