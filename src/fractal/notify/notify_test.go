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
	noPrime := NewNotifier("MyService", "MyServiceInstance", true, false, 100, endpoints[0])
	noSecond := NewNotifier("MyService", "MyServiceInstance", true, false, 100, endpoints[1:]...)

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
