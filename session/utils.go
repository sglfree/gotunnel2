package session

import (
  "fmt"
)

var DEBUG = false

func info(format string, args ...interface{}) {
  if DEBUG {
    fmt.Printf(format, args...)
  }
}
