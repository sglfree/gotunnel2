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
  "time"
  "crypto/aes"
  "errors"
  "math/rand"
)

func init() {
  rand.Seed(time.Now().UnixNano())
}

const (
  CONFIG_FILENAME = ".gotunnel.conf"

  sigClose = uint8(0)
  sigPing = uint8(1)

  keepaliveSessionMagic = "I am a keepalive session."
)

var (
  PING_INTERVAL = time.Second * 5
  BAD_CONN_THRESHOLD = time.Second * 15
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

func formatFlow(n uint64) string {
  units := []string{"b", "k", "m", "g", "t"}
  i := 0
  ret := ""
  for n > 0 && i < len(units) {
    if n % 1024 > 0 {
      ret = fmt.Sprintf("%d%s", n % 1024, units[i]) + ret
    }
    n = n / 1024
    i += 1
  }
  return ret
}

func encrypt(key []byte, in []byte) ([]byte, error) {
  if len(in) % aes.BlockSize != 0 {
    return nil, errors.New("input data length incorrect")
  }
  block, err := aes.NewCipher(key)
  if err != nil { return nil, err }
  out := make([]byte, len(in))
  for i, l := 0, len(in); i < l; i += aes.BlockSize {
    block.Encrypt(out[i : i + aes.BlockSize], in[i : i + aes.BlockSize])
  }
  return out, nil
}

func genRandBytes(n int) []byte {
  buf := new(bytes.Buffer)
  buf.Grow(n)
  for i := 0; i < n; i++ {
    buf.Write([]byte{byte(rand.Intn(256))})
  }
  return buf.Bytes()
}

//func main() {
//  println(formatFlow(5))
//  println(formatFlow(1024))
//  println(formatFlow(1025))
//  println(formatFlow(1024 * 24))
//  println(formatFlow(1024 * 1024))
//  println(formatFlow(1024 * 1025))
//  println(formatFlow(1024 * 1024 * 1024))
//  println(formatFlow(1024 * 1024 * 1025 + 1024 * 48))
//  println(formatFlow(1024 * 1024 * 1025 + 1024 * 48 + 3))

//  key := genRandBytes(24)
//  in := genRandBytes(64)
//  enc, err := encrypt(key, in)
//  if err != nil { log.Fatal(err) }
//  fmt.Printf("%x\n%x\n", in, enc)
//}
