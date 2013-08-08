package main

import (
  box "github.com/nsf/termbox-go"
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
  "reflect"
  "sort"
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

// from godit
func rune_width(r rune) int {
  if r >= 0x1100 &&
  (r <= 0x115f || r == 0x2329 || r == 0x232a ||
  (r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
  (r >= 0xac00 && r <= 0xd7a3) ||
  (r >= 0xf900 && r <= 0xfaff) ||
  (r >= 0xfe30 && r <= 0xfe6f) ||
  (r >= 0xff00 && r <= 0xff60) ||
  (r >= 0xffe0 && r <= 0xffe6) ||
  (r >= 0x20000 && r <= 0x2fffd) ||
  (r >= 0x30000 && r <= 0x3fffd)) {
    return 2
  }
  return 1
}

type Printer struct {
  x int
  y int
  w int
  h int
  lineWidth int
}

func NewPrinter(lineWidth int) *Printer {
  ret := &Printer{
    lineWidth: lineWidth,
  }
  ret.w, ret.h = box.Size()
  return ret
}

func (self *Printer) Reset() {
  self.x = 0
  self.y = 0
  self.w, self.h = box.Size()
}

func (self *Printer) Print(format string, args ...interface{}) {
  x := self.x
  for _, c := range fmt.Sprintf(format, args...) {
    box.SetCell(x, self.y, c, box.ColorWhite, box.ColorDefault)
    x += rune_width(c)
  }
  self.y += 1
  if self.y >= self.h {
    self.y = 0
    self.x += self.lineWidth
  }
}

type sortByValue struct {
  m interface{}
  key reflect.Value
  fun func(a, b reflect.Value) bool
}

func (self *sortByValue) Len() int {
  return reflect.ValueOf(self.m).Len()
}

func (self *sortByValue) Less(i, j int) bool {
  v := reflect.ValueOf(self.m)
  return self.fun(v.MapIndex(self.key.Index(i)),
  v.MapIndex(self.key.Index(j)))
}

func (self *sortByValue) Swap(i, j int) {
  tmp := reflect.ValueOf(self.key.Index(i).Interface())
  self.key.Index(i).Set(self.key.Index(j))
  self.key.Index(j).Set(tmp)
}

func ByValue(m interface{}, fun func(a, b reflect.Value) bool) reflect.Value {
  sm := new(sortByValue)
  sm.m = m
  sm.fun = fun
  t := reflect.TypeOf(m)
  v := reflect.ValueOf(m)
  sm.key = reflect.MakeSlice(reflect.SliceOf(t.Key()), 0, v.Len())
  for _, key := range v.MapKeys() {
    sm.key = reflect.Append(sm.key, key)
  }
  sort.Sort(sm)
  return sm.key
}
