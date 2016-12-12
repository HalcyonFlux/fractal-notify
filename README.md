# fractal/notes

`notes` is a simplistic notification/logging service used in HalcyonFlux/fractal.
Errors and messages are sent into a `chan <- *notes.Note` channel and get written
by a looping goroutine.

```go
package main

import (
  "fractal/notes"
  "time"
)

func main() {

  logger, noteChan := notes.NewLogger("MyService","MyServiceInstance","/var/logs/myservice/myservice.log",2,100)

  // Write arbitrary messages and error logs
  go func(){
    for i := 0; i < 100;i++{
      iteration := strconv.Itoa(i)+"th iteration"

      if i % 2 == 0 {
        noteChan <- &notes.Note{"MyGoRoutine",notes.New(1,iteration+": something bad happened")}
      }else{
        noteChan <- &notes.Note{"MyGoRoutine",iteration+": normalMessage"}
      }

      time.Sleep(50*time.Millisecond)
    }
  }

  // Close logger after 5 seconds
  go func() {
    <- time.After(5*time.Second)
    close(noteChan)
  }

  // Fetch messages and errors
  logger() // blocks
  // go logger() // does not block

}

```
