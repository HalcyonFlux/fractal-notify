package notify

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func ignoreStdOut(t *testing.T) *os.File {
	old := os.Stdout
	fPtr, err := os.Open(os.DevNull)
	defer fPtr.Close()

	if err != nil {
		t.Error("Cannot open " + os.DevNull)
	}

	os.Stdout = fPtr

	return old
}

// Readme use case
func sTestNotify(t *testing.T) {

	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

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

func sTestSetCodes(t *testing.T) {

	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	codesOK := map[int][2]string{
		2:   [2]string{"ERR", "Woopsie"},
		5:   [2]string{"WRN", "BeCareful"},
		404: [2]string{"ERR", "NotFound"},
		8:   [2]string{"ERR", "TotalFailure"},
	}

	codesBad1 := map[int][2]string{
		1:   [2]string{"ERR", "Woopsie"},
		5:   [2]string{"WRN", "BeCareful"},
		404: [2]string{"ERR", "NotFound"},
		8:   [2]string{"ERR", "TotalFailure"},
	}

	codesBad2 := map[int][2]string{
		2:   [2]string{"ERR", "Woopsie"},
		5:   [2]string{"WRN", "BeCareful"},
		404: [2]string{"ERR", "NotFound"},
		999: [2]string{"ERR", "TotalFailure"},
	}

	tests := []struct {
		newCodes map[int][2]string
		err      bool
	}{
		{codesOK, false},
		{codesBad1, true},
		{codesBad2, true},
		{codesOK, true}, // Will set codes twice and fail
	}

	for i, test := range tests {
		notifier := NewNotifier("MyService", "MyServiceInstance", true, false, 100)
		if i == 3 {
			notifier.SetCodes(test.newCodes)
		}
		if err := notifier.SetCodes(test.newCodes); (err != nil) != test.err {
			if err != nil {
				t.Error("SetCodes " + strconv.Itoa(i) + "th test failed: " + err.Error())
			} else {
				t.Error("SetCodes " + strconv.Itoa(i) + "th test failed")
			}
		}
	}
}

func sTestExitWithoutRunning(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, 100)

	if err := notifier.Exit(); err == nil {
		t.Error("Inactive notifier should complain about exiting")
	}
}

func sTestExitBacklog(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, 100)
	send := notifier.Sender("TestExitBacklog")

	for i := 1; i <= 50; i++ {
		send("Creating backlog")
	}

	if len(notifier.noteChan) == 0 {
		t.Error("Exit: failed setting upt the test. noteChan is empty")
	}

	notifier.Exit() // wait for backlog to clear, then exit

	if len(notifier.noteChan) == 0 {
		t.Error("Exit: backlog should not be cleared before exiting")
	}

}

func sTestExitBacklog2(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, 100)
	send := notifier.Sender("TestExitBacklog2")

	for i := 1; i <= 50; i++ {
		send("Creating backlog")
	}

	if len(notifier.noteChan) == 0 {
		t.Error("Exit: failed setting upt the test. noteChan is empty")
	}

	go notifier.Run()
	for !notifier.IsReady() {
		// wait for logging to start
	}

	notifier.Exit() // wait for backlog to clear, then exit

	if len(notifier.noteChan) > 0 {
		t.Error("Exit: backlog should be cleared before exiting")
	}

}

func sTestUnsuportedEndpoint(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	badEndPoint := struct {
		file string
	}{
		file: "mylog.log",
	}

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, 100, badEndPoint)
	go notifier.Run()
	time.Sleep(50 * time.Millisecond)
	notifier.Exit()
}

func sTestNegativeErrorCode(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, 100)
	fail := notifier.Failure("TestNegativeErrorCode")
	err := fail(-1, "Illegal code")
	if !IsCode(1, err) {
		t.Error("newf: Did not change negative code to 1")
	}
}

func TestUnsupportedValue(t *testing.T) {

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Error("send: failed to setup test: " + err.Error())
	}
	os.Stdout = w
	defer func() { w.Close(); os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, 100)
	send := notifier.Sender("TestUnsupportedValue")

	badValue := struct {
		greet string
	}{
		greet: "Hello, World!",
	}

	go notifier.Run()
	send(badValue)

	for !notifier.IsReady() {
		// wait for logging to start
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)

	response := strings.Split(buf.String(), "\t")

	if len(response) != 8 {
		t.Error("log: response does not contain 8 fields: " + strconv.Itoa(len(response)))
	}

	if response[6] != "999" || response[7] != "Unknown value used in notify.send" {
		t.Error("log: bad response to an unsupported value:" + strings.Join(response, " "))
	}
}
