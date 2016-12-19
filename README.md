# fractal-notify

`notify` is a simplistic notification/logging service used in `HalcyonFlux/fractal`.
A new `notifier` is instantiated by running the `notify.NewNotifier()` command.
There are no limits as to how many notifiers can be created, but the recommended
way of using `notify` is to instantiate a single notifier and then specify the
log-files (or other \*os.File instances) depending on the use case (e.g. using
`os.Stdout` in debug mode). In order to ease log-analysis, each distinct element of
the application (e.g. client, server, etc.) should be given its own send and fail
functions (created with `notifier.Sender` and `notifier.Failure` respectively).

`notify` can deal with any implementation of the `errors.error` interface as well
as simple string messages. A function created by `notifier.Failure()` can replace
`errors.New()` with the additional benefit of logging errors. The type
`notify.notification` implements the standard error interface and can be used
wherever type `error` is appropriate. The `notify.IsCode()` function can be used
to safely verify that the provided error has a specific code.

## API

`notify` exposes a minimal API, consisting of following functions:

* Package functions:
  * `NewNotifier(service string, instance string, logAll bool, async bool, json bool, notifierCap int, files ...interface{}) *notifier` - creates a new instance of notifier.
    * `service` - a name of the service using notify.
    * `instance` - a name of the instance of the service using notify. Both service and instance are used to ease
    log-analysis if notify is being used by several programs/nodes.
    * `logAll` - a flag of whether messages sent by a fail AND send commands should be logged. If set to false, only the fail-notifications will be logged.
    * `async` - a flag of whether send and fail should write to the log asynchronously. If set to false, the send and fail commands might block until the backlog of notifications is cleared.
    * `json` - a flag of whether the notifications should be written as json objects. If set to false, a tab-separated line with 8 fields will be written for each notification.
    * `notifierCap` - specifies the size the notes channel buffer. Applications with large notification streams
    should use bigger buffers to avoid blocking by synchronized notifiers.
    * `files` - a (variadic) list of file refrences and \*os.File instances to which notifications should be written.
  * `IsCode(code int, err error) bool` - verifies whether an error instance has a specific the notification code
    * `code` - the presumed error code
    * `err` - an instance of error
* Notifier methods:
  * `(no *notifier) Sender(sender string) func(interface{}) error` - creates a personalized function to log notifications/errors. A "send" command logs and returns the same error, if an error has been passed to it.
    * `sender` - a name of the sending entity (e.g. client, server, etc.).
  * `(no *notifier) Failure(sender string) func(int, string, ...interface{}) error` - creates a personalized
  * function to log errors. `send(fail(...))` is a redundant, but legal command.
    * `sender` - a name of the failing entity (e.g. client, server, etc.).
  * `(no *notifier) SetCodes(newCodes map[int][2]string) error` - replaces the built in codes with application-specific codes
    * `newCodes` - a map containing all or some replacements for the built-in error codes. See `notify_codes.go` for the built-in map.
  * `(no *notifier) Run()` - starts the logging service. This command will block. Run in a goroutine to avoid blocking. If started as a goroutine, it is a good idea to let the notifier warmup before continuing.
  * `(no *notifier) WarmUp()` - waits untill the notifier is ready to write incoming notifications to log.
  * `(no *notifier) Exit()` - stops a running notifier (exits a blocking `notifier.Run()` command) and closes all files.
* Notification methods:
  * `Error()` - returns the notification/error message.

## Displaying and logging notifications

An application using `notify` has to instantiate at least one notifier and use
at least one personalized send and/or fail function to notify the user and log
notifications:

```go
// log failures asynchronously as tab-separated text messages to stdout. The buffer contains 100 notifications.
notifier := notify.NewNotifier("MyService","MyServiceInstance","/var/logs/myservice/myservice.log",false,true,false,100)
send := notifier.Sender("server")
fail := notifier.Failure("server")
// ...
// ...
// ...
send("Verifying that x <= 5")
if x > 5 {
  return fail(1,"Bad value (x>5)") // will log a general error and return an error instance.
}
// ...
```

## Endpoints

A notifier can have any number of endpoints it'll send notes to. A valid endpoint
is a filename (string) or any type implementing the `os.File` interface. One good
use case of defining several endpoints is writing notifications to a file and
simultaneously outputing them to the standard output (`os.Stdout`), e.g.:

```go
notifie := notify.NewNotifier("greeter","node_1",true,true,false,100,"~/world.log",os.Stdout)
go notifier.Run()
notifier.WarmUp() // will block until notifier is ready

send := notifier.Sender("friend")

send("Hello")
send("World!")

notifier.Exit()
```

Not specifying an endpoint will result in all notifications being written to `os.Stdout`.

`notify` also plays well with `fractal/beacon`, i.e. logs and messages can be
sent directly to a remote subscriber (e.g. log-aggregator). See `fractal/beacon`
for details.

## notify.New instead of errors.New

Replacing `errors.New` with a personalized command created by `notify.Failure`
allows for simultaneous creation and logging of errors/notifications. Most use
cases of `notify` are shown in the small go-program bellow:

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

	// Instantiate the notifiers
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
1482142395	MyService	MyServiceInstance	MyMainThread	MSG	0	GeneralMessage	MyMainThread is done checking myfunc.cfg
1482142395	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	MyGoRoutine will start logging now!
1482142395	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	1th iteration: nothing bad happened
1482142395	MyService	MyServiceInstance	MyGoRoutine	ERR	3	FailedAction	2th iteration: something bad happened  -> [main.go: 40]
1482142396	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	3th iteration: nothing bad happened
1482142396	MyService	MyServiceInstance	MyGoRoutine	ERR	3	FailedAction	4th iteration: something bad happened  -> [main.go: 40]
1482142397	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	5th iteration: nothing bad happened
1482142397	MyService	MyServiceInstance	MyGoRoutine	ERR	1	GeneralError	6th iteration: something bad happened  -> [main.go: 38]
1482142398	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	7th iteration: nothing bad happened
1482142398	MyService	MyServiceInstance	MyGoRoutine	ERR	3	FailedAction	8th iteration: something bad happened  -> [main.go: 40]
1482142399	MyService	MyServiceInstance	MyGoRoutine	MSG	0	GeneralMessage	9th iteration: nothing bad happened
1482142399	MyService	MyServiceInstance	MyGoRoutine	ERR	3	FailedAction	10th iteration: something bad happened  -> [main.go: 40]
1482142400	MyService	MyServiceInstance	notifier	MSG	0	GeneralMessage	Exit() command has been executed. Stopping the notification service.
```

and

```plain
1482142395	MyService	MyServiceInstance	MyMainThread	ERR	3	FailedAction	Could not open myfunc.cfg: open /var/opt/myfunc.cfg: no such file or directory  -> [main.go: 54]
1482142395	MyService	MyServiceInstance	MyMainThread	MSG	0	GeneralMessage	Trying to open myfunc.cfg  -> [main.go: 52]
1482142400	MyService	MyServiceInstance	notifier	MSG	0	GeneralMessage	Exit() command has been executed. Stopping the notification service.
```

These log files can be easily analyzed by filtering error codes, senders and so on.

## notify.New AND errors.New

It is possible to use both `errors.New` and `notify.Failure` to manage program workflow.
`notify.IsCode()` allows for a safe comparison of codes. All implementations of
`errors.error`, except `notify.notification`, default to code=1

```go
fail := notifier.Failure("client")
for _,err := range []error{errors.New("Woops"), fail(3,"Ouch")} {

  switch {

  case notify.IsCode(err,1):
    fmt.Println("Simple general error") // errrors.New will land here

  case notify.IsCode(err,3):
    fmt.Println("Something has failed") // fail will land here

  case 999:
    fmt.Println("Oops: ",err.Error()) // UnintendedCase

  default:
    fmt.Println("hmm") // errors.New would land here if case=1 was omitted.

  }
}
```

## Many notifiers

Even though it is possible to use several `notifiers` in a programm, in most
cases it is not necessary. The best approach of using `notify` is to create a
single notifier that writes to the main log-file and, if the program is running
in debugging mode, to the stdout, and giving each distinct package or element of
the program (client, server, etc.) its own send and fail functions, e.g.:

```go
endpoints := []interface{}{"mylogfile.log"}
if os.Getenv("MYSERVICE_DEBUG") == "1" {
	endpoints = append(endpoints, os.Stdout)
}

notifier := notify.NewNotifier("MyService", "MyServiceInstance", false, false, false, 100, endpoints...)

client := mypackage.Client()
clientFail := notifier.Failure("client")

server := mypackage.Server()
clientFail := notifier.Failure("server")

```

The easiest way of following this advice is to pass along a pointer to the
notifier instance. Any element that wants to write to log can create a sender
ad hoc.

```go
package mypackage

import(
  "fractal/notify"
)

type Client struct {
  // ...
  // ...
  // ...
	send     func(value interface{}) error // dedicated sender
}

notifier := notify.NewNotifier("MyService", "MyServiceInstance", false, false, false, 100)
client := Client{notifier.Sender("client")}
/* alternatively:
client := Client{}
client.send = notifier.Sender("client")
*/
client.send("Hello, World!")
```
