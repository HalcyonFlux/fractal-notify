package notify

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestNotify(t *testing.T) {

	// Instantiate the notifier(s)
	noMain, noMainChan := NewNotifier("MyService", "MyServiceInstance", os.Getenv("HOME")+"/myservice/myservice_main.log", 1, 100)          // myservice_main.log
	noSupport, noSupportChan := NewNotifier("MyService", "MyServiceInstance", os.Getenv("HOME")+"/myservice/myservice_support.log", 1, 100) // myservice_support.log

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
