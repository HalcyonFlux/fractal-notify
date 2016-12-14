package notify

import (
	"errors"
	"os"
	"strconv"
	"testing"
	"time"
)

// Readme use case
func TestNotify(t *testing.T) {

	// Define relevant endpoints
	endpoints := []interface{}{
		os.Getenv("HOME") + "/myservice/myservice_main.log",
		os.Getenv("HOME") + "/myservice/myservice_support.log",
		os.Stdout,
	}

	// Instantiate the notifier(s)
	noPrime := NewNotifier("MyService", "MyServiceInstance", true, 100, endpoints[0])
	noSecond := NewNotifier("MyService", "MyServiceInstance", true, 100, endpoints[1], endpoints[2])

	// Run the notification service in the background to avoid blocking
	go noSecond.Run()

	// Write arbitrary messages and error logs
	go func() {

		sendMsg := noPrime.Sender("MyGoRoutine")  // personalized plain message sender
		sendErr := noPrime.Failure("MyGoRoutine") // personalized notification sender

		for i := 1; i <= 100; i++ {

			if i%2 == 0 {
				sendMsg(errors.New(strconv.Itoa(i) + "th iteration: something bad happened")) // will log (and return) an error
			} else {
				sendMsg(strconv.Itoa(i) + "th iteration: nothing bad happened") // will log a message
			}

			time.Sleep(500 * time.Millisecond)
		}
		sendErr(3, "MyGoRoutine will quit now!") // will log (and return) an error
	}()

	// Send error notifications to the other notifier
	newErr := noSecond.Failure("MyMainThread")
	if f, err := os.Open("/var/opt/myfunc.cfg"); err != nil {
		newErr(3, "Could not open myfunc.cfg: "+err.Error()) // will log (and return) an error
	} else {
		f.Close()
	}

	// Use any notifier you want
	sendToMain := noPrime.Sender("MyMainThread")           // use main notifier for other tasks too
	sendToMain("MyMainThread is done checking myfunc.cfg") // will appear in myservice_main.log

	// Close notifier after 5 seconds
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
