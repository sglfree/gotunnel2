package main

import (
  "io/ioutil"
  "os/user"
  "log"
  "path/filepath"
  //"strings"
  "encoding/json"
  "bytes"
  "os"
  "fmt"
)

const (
  CONFIG_FILENAME = ".gotunnel.conf"

  sigClose = uint8(0)
  sigPing = uint8(1)

  keepaliveSessionMagic = "I am a keepalive session."
  obfusSessionMagic = "I am a obfuscation session."
)

func loadConfig(defaultConf map[string]string) map[string]string {
  currentUser, err := user.Current()
  if err != nil { log.Fatal("cannot get current user") }
  configFilePath := filepath.Join(currentUser.HomeDir, CONFIG_FILENAME)
  fmt.Printf("loading configuration from %s\n", configFilePath)
  s, err := ioutil.ReadFile(configFilePath)
  if err != nil {
    //if !strings.Contains(err.Error(), "no such file") { log.Fatal(err) }
    err = ioutil.WriteFile(configFilePath, marshalConfig(defaultConf), os.ModePerm)
    return defaultConf
  }
  config := make(map[string]string)
  err = json.Unmarshal(s, &config)
  if err != nil { log.Fatal("config file parse error") }
  return config
}

func saveConfig(conf map[string]string) {
  currentUser, err := user.Current()
  if err != nil { log.Fatal("cannot get current user") }
  configFilePath := filepath.Join(currentUser.HomeDir, CONFIG_FILENAME)
  err = ioutil.WriteFile(configFilePath, marshalConfig(conf), os.ModePerm)
  if err != nil { log.Fatal("cannot write config file") }
}

func marshalConfig(config map[string]string) []byte {
  b, err := json.Marshal(config)
  if err != nil { log.Fatal("cannot marshal config") }
  buf := new(bytes.Buffer)
  json.Indent(buf, b, "", "    ")
  return buf.Bytes()
}
