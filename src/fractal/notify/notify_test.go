package notify

import (
	"os"
	"strconv"
	"testing"
	"time"
)

// Readme use case
func TestNotify(t *testing.T) {

	// Select relevant endpoints
	endpoints := []interface{}{
		os.Getenv("HOME") + "/myservice/myservice_main.log",
		os.Getenv("HOME") + "/myservice/myservice_support.log",
		os.Stdout,
	}

	// Instantiate the notifier(s)
	noMain, noMainChan := NewNotifier("MyService", "MyServiceInstance", true, 100, endpoints[0])
	noSupport, noSupportChan := NewNotifier("MyService", "MyServiceInstance", true, 100, endpoints[1], endpoints[2])

	// Run notification service in the background to avoid blocking
	go noSupport.Loop()

	// Create a personalized sender
	send := noMain.PersonalizeSend("MyGoRoutine")

	// Write arbitrary messages and error logs
	go func() {
		for i := 1; i <= 100; i++ {

			if i%2 == 0 {
				send(New(1, strconv.Itoa(i)+"th iteration: something bad happened"))
			} else {
				send(strconv.Itoa(i) + "th iteration: nothing bad happened")
			}

			time.Sleep(500 * time.Millisecond)
		}
		New(3, "MyGoRoutine will quit now!") // will not log
	}()

	// Alternatively, use a personalized notify.New function
	newErr := noSupport.PersonalizeNew("MyMainThread")
	if f, err := os.Open("/var/opt/myfunc.cfg"); err != nil {
		newErr(3, "Could not open myfunc.cfg: "+err.Error()) // Will automatically log
	} else {
		f.Close()
	}

	// Use any notifier you want
	sendToMain := noMain.PersonalizeSend("MyMainThread")   // use main notifier for other tasks too
	sendToMain("MyMainThread is done checking myfunc.cfg") // Will appear in myservice_main.log

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
