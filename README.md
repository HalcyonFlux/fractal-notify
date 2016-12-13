# fractal/notify

`notify` is a simplistic notification/logging service used in `HalcyonFlux/fractal`.
Errors and messages are sent into a `chan <- *notify.Note` channel and get written
by a blocking loop (or a non-blocking goroutine) to a prespecified log file.

`notify` can deal with any implementation of the `errors.error` interface as well
as simple string messages. We recommend using `notify.Send` (and derivatives)
in combination with `notify.Notification` instead of plain `errors.New()`.

A `notify.Notification` implements the standard error interface and can be used
wherever type `error` is appropriate.
`notify.New()`, `notify.Newf()`, `notify.Send()`, as well as a personalized send
fuction (created via `notifier.PersonalizeSend()`) return a `notify.Notification`
struct with a message and an error/notification code. You can use the `notify.IsCode()`
function to safely verify that the provided error has a specific code.

 ## Displaying and logging notifications

To send a note to an existing notifier, one can use `notify.Send`:

```go
if x > 5 {
  notify.Send("server","x should not be greater than 5",noteChan)
  return notify.New(1,"Bad value (x>5)")
}
```

However if an error was supplied to`notify.Send`, it will return the same error
back, which allows for a simplified syntax:

```go
if x > 5 {
  return notify.Send("server",notify.New(1,"Bad value (x>5)"),noteChan)   
}
```

The most comfortable way of using `notify` is by creating a personalized Send
function, e.g:

```go
notifier, noteChan := notify.NewNotifier("MyService","MyServiceInstance","/var/logs/myservice/myservice.log",2,100)
send := notifier.PersonalizeSend("server")
// ...
// ...
// ...
if x > 5 {
  return send(notify.New(1,"Bad value (x>5)"))   
}
```

## notify.New instead of errors.New

To log all errors and warnings, one could replace `errors.New` with `notify.New`:

```go
package main

import (
	"fractal/notify"
	"os"
	"strconv"
	"time"
)

func main() {

	// Instantiate the notifier(s)
	noMain, noMainChan := notify.NewNotifier("MyService", "MyServiceInstance", os.Getenv("HOME")+"/myservice/myservice_main.log", 1, 100)          // myservice_main.log
	noSupport, noSupportChan := notify.NewNotifier("MyService", "MyServiceInstance", os.Getenv("HOME")+"/myservice/myservice_support.log", 1, 100) // myservice_support.log

	// Run notification service in the background to avoid blocking
	go noSupport.Loop()

	// Create a personalized sender
	send := noMain.PersonalizeSend("MyGoRoutine")

	// Write arbitrary messages and error logs
	go func() {
		for i := 1; i <= 100; i++ {

			if i%2 == 0 {
				send(notify.New(1, strconv.Itoa(i)+"th iteration: something bad happened"))
			} else {
				send(strconv.Itoa(i) + "th iteration: nothing bad happened")
			}

			time.Sleep(500 * time.Millisecond)
		}
		notify.New(3, "MyGoRoutine will quit now!") // will not log
	}()

	// Alternatively, use a personalized notify.New function
	newErr := noSupport.PersonalizeNew("MyMainThread")
	if f, err := os.Open("/var/opt/myfunc.cfg"); err != nil {
		newErr(3, "Could not open myfunc.cfg: "+err.Error()) // Will automatically log
	} else {
		f.Close()
	}

	// Close notifier after 5 seconds
	go func() {
		<-time.After(5 * time.Second)
		close(noMainChan)
	}()

	// Fetch messages and errors from the first notifier
	noMain.Loop() // blocks

	// Close the second notifier and end program
	close(noSupportChan)
}
```

This example generates following log entries:

```plain
1481649734	MyService	MyServiceInstance	MyGoRoutine	MSG	0	NormalMessage	1th iteration: nothing bad happened
1481649735	MyService	MyServiceInstance	MyGoRoutine	ERR	1	GeneralError	2th iteration: something bad happened
1481649735	MyService	MyServiceInstance	MyGoRoutine	MSG	0	NormalMessage	3th iteration: nothing bad happened
1481649736	MyService	MyServiceInstance	MyGoRoutine	ERR	1	GeneralError	4th iteration: something bad happened
1481649736	MyService	MyServiceInstance	MyGoRoutine	MSG	0	NormalMessage	5th iteration: nothing bad happened
1481649737	MyService	MyServiceInstance	MyGoRoutine	ERR	1	GeneralError	6th iteration: something bad happened
1481649737	MyService	MyServiceInstance	MyGoRoutine	MSG	0	NormalMessage	7th iteration: nothing bad happened
1481649738	MyService	MyServiceInstance	MyGoRoutine	ERR	1	GeneralError	8th iteration: something bad happened
1481649738	MyService	MyServiceInstance	MyGoRoutine	MSG	0	NormalMessage	9th iteration: nothing bad happened
1481649739	MyService	MyServiceInstance	MyGoRoutine	ERR	1	GeneralError	10th iteration: something bad happened
1481649739	MyService	MyServiceInstance	notifier	MSG	0	GeneralMessage	Note channel has been closed. Exiting notifier loop.
```

and

```plain
1481649734	MyService	MyServiceInstance	MyMainThread	ERR	3	FailedAction	Could not open myfunc.cfg: open /var/opt/myfunc.cfg: no such file or directory
1481649739	MyService	MyServiceInstance	notifier	MSG	0	GeneralMessage	Note channel has been closed. Exiting notifier loop.
```

## notify.New AND errors.New

It is possible to use both `errors.New` and `notify.New` to manage program workflow.
`notify.IsCode()` allows for save comparison of codes. All implementations of
`errors.error`, except `notify.Notification` default to code=1

```go
for _,err := range []error{errors.New("Woops"), notify.New(3,"")} {

  switch {

  case notify.IsCode(err,1):
    fmt.Println("Simple general error") // errrors.New will land here

  case notify.IsCode(err,3):
    fmt.Println("Something has failed") // notify.New will land here

  case 999:
    fmt.Println("Oops: ",err.Message)

  default:
    fmt.Println("hmm") // errors.New would land here if case=1 was omitted.

  }
}
```
