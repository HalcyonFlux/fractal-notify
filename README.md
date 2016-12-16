# fractal/notify

`notify` is a simplistic notification/logging service used in `HalcyonFlux/fractal`.
Errors and messages are sent into a `chan <- *notify.Note` channel and get written
to a prespecified log file by a blocking loop (or a non-blocking goroutine).

`notify` can deal with any implementation of the `errors.error` interface as well
as simple string messages. We recommend using `notify.Send` (and derivatives)
in combination with `notify.Notification` instead of plain `errors.New()`.

The type `notify.Notification` implements the standard error interface and can be used
wherever type `error` is appropriate. `notify.New()`, `notify.Newf()`, `notify.Send()`,
as well as the personalized Send (created by `notifier.PersonalizeSend()`) and New
(created by `notifier.PersonalizeNew()`) fuctions return a `notify.Notification`
struct with a message and an error/notification code. One can use the `notify.IsCode()`
function to safely verify that the provided error has a specific code.


## Basics

### Displaying and logging notifications


```go
notifier := notify.NewNotifier("MyService","MyServiceInstance","/var/logs/myservice/myservice.log",false,true,false,2,100)
send := notifier.Sender("server")
fail := notifier.Failure("server")
// ...
// ...
// ...
send("Verifying that x <= 5")
if x > 5 {
  return fail(1,"Bad value (x>5)")
}
// ...

```

### Endpoints

A notifier can have any number of endpoints he'll send notes to. A valid endpoint
is a filename (string) or any type implementing the `os.File` interface. One good
use case of using several endpoints is writing logs to a file and simultaneously
outputing to the standard output (`os.Stdout`), e.g.:

```go
notifie := notify.NewNotifier("friend","greeter 1",true,100,"~/world.log", os.Stdout)
go notifier.Run()
notifier.WarmUp() // will block until notifier is ready

send := notifier.Sender("friend")

send("Hello")
send("World!")

notifier.Exit()
```

`notify` also plays well with `fractal/beacon`, i.e. logs and messages can be
sent directly to a remote subscriber (e.g. log-aggregator).

## notify.New instead of errors.New

Replacing `errors.New` with `notify.New`, or to be more specific - with `notify.Send` -
allows for simultaneous creation and logging of errors/notifications. Most use cases of
`notify` are shown in the small go-program bellow:

```go
package main

import (
	"errors"
	"fractal/notify"
	"os"
	"strconv"
	"time"
)

func main() {

	// Define relevant endpoints
	endpoints := []interface{}{
		os.Getenv("HOME") + "/myservice/myservice_main.log",
		os.Getenv("HOME") + "/myservice/myservice_support.log",
		os.Stdout,
	}

	// Instantiate the notifier(s)
	noPrime := notify.NewNotifier("MyService", "MyServiceInstance", true, false, false, 100, endpoints[0])
	noSecond := notify.NewNotifier("MyService", "MyServiceInstance", true, true, false, 100, endpoints[1:]...)

	// Run the secondary notification service in the background to avoid blocking
	go noSecond.Run()

	// Write arbitrary messages and error logs
	go func() {

		send := noPrime.Sender("MyGoRoutine")  // personalized plain message sender
		fail := noPrime.Failure("MyGoRoutine") // personalized notification sender

		send("MyGoRoutine will start logging now!") // will log a GeneralMessage
		for i := 1; i <= 100; i++ {

			if i%2 == 0 {
				if i%3 == 0 {
					send(errors.New(strconv.Itoa(i) + "th iteration: something bad happened")) // will log (and return) an error with code=1
				} else {
					fail(3, strconv.Itoa(i)+"th iteration: something bad happened") // will log (and return) an error with code=3
				}
			} else {
				send(strconv.Itoa(i) + "th iteration: nothing bad happened") // will log a message
			}

			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Send error notifications to the other notifier
	failSecond := noSecond.Failure("MyMainThread") // personalized fail function for the second notifier
	failSecond(0, "Trying to open myfunc.cfg")     // will log a GeneralMessage
	if f, err := os.Open("/var/opt/myfunc.cfg"); err != nil {
		failSecond(3, "Could not open myfunc.cfg: %s", err.Error()) // will log (and return) an error
	} else {
		f.Close()
	}

	// Use any notifier you want
	sendToMain := noPrime.Sender("MyMainThread")           // use main notifier for other tasks too
	sendToMain("MyMainThread is done checking myfunc.cfg") // will appear in myservice_main.log

	// Close main notifier after 5 seconds
	go func() {
		<-time.After(5 * time.Second)
		noPrime.Exit()
	}()

	// Try to send to a closed notifier after 6 seconds
	go func() {
		<-time.After(6 * time.Second)
		sendToMain("Are you still there?") // will not be sent
	}()

	// Fetch messages and errors from the first notifier
	noPrime.Run() // blocks for five seconds

	// Close the second notifier and end program
	noSecond.Exit()
}
```

This small example generates following log entries:

```plain
1481905460	MyService	MyServiceInstance	MyMainThread	MSG	0	GeneralMessage	MyMainThread is done checking myfunc.cfg
1481905460	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	MyGoRoutine will start logging now!
1481905460	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	1th iteration: nothing bad happened
1481905460	MyService	MyServiceInstance	MyGoRoutine	ERR	3	FailedAction	2th iteration: something bad happened  -> [main.go: 40]
1481905461	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	3th iteration: nothing bad happened
1481905461	MyService	MyServiceInstance	MyGoRoutine	ERR	3	FailedAction	4th iteration: something bad happened  -> [main.go: 40]
1481905462	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	5th iteration: nothing bad happened
1481905462	MyService	MyServiceInstance	MyGoRoutine	ERR	1	GeneralError	6th iteration: something bad happened  -> [main.go: 38]
1481905463	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	7th iteration: nothing bad happened
1481905463	MyService	MyServiceInstance	MyGoRoutine	ERR	3	FailedAction	8th iteration: something bad happened  -> [main.go: 40]
1481905464	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	9th iteration: nothing bad happened
1481905464	MyService	MyServiceInstance	MyGoRoutine	ERR	3	FailedAction	10th iteration: something bad happened  -> [main.go: 40]
1481905465	MyService	MyServiceInstance	notifier	MSG	0	GeneralMessage	Exit() command has been executed. Stopping the notification. service
```

and

```plain
1481905460	MyService	MyServiceInstance	MyMainThread	ERR	3	FailedAction	Could not open myfunc.cfg: open /var/opt/myfunc.cfg: no such file or directory  -> [main.go: 54]
1481905460	MyService	MyServiceInstance	MyMainThread	MSG	0	GeneralMessage	Trying to open myfunc.cfg  -> [main.go: 52]
1481905465	MyService	MyServiceInstance	notifier	MSG	0	GeneralMessage	Exit() command has been executed. Stopping the notification service.
```

## notify.New AND errors.New

It is possible to use both `errors.New` and `notify.New` to manage program workflow.
`notify.IsCode()` allows for save comparison of codes. All implementations of
`errors.error`, except `notify.Notification`, default to code=1

```go
for _,err := range []error{errors.New("Woops"), notify.New(3,"Ouch")} {

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
