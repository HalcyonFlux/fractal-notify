package notify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
func TestNotify(t *testing.T) {

	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	// Define relevant endpoints
	endpoints := []interface{}{
		os.Getenv("HOME") + "/myservice/myservice_main.log",
		os.Getenv("HOME") + "/myservice/myservice_support.log",
		os.Stdout,
	}

	// Instantiate the notifier(s)
	noPrime := NewNotifier("MyService", "MyServiceInstance", true, false, false, 100, endpoints[0])
	noSecond := NewNotifier("MyService", "MyServiceInstance", true, false, false, 100, endpoints[1:]...)

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

func TestSetCodes(t *testing.T) {

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
		{codesOK, true}, // Cannot set if notifier is already running
	}

	for i, test := range tests {
		notifier := NewNotifier("MyService", "MyServiceInstance", true, false, false, 100)
		if i == 3 {
			go notifier.Run()
			for !notifier.isReady() {
				// wait for start
			}
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

func TestExitWithoutRunning(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, false, 100)

	if err := notifier.Exit(); err == nil {
		t.Error("Inactive notifier should complain about exiting")
	}
}

func TestExitBacklog(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, false, 100)
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

func TestExitBacklog2(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, false, false, 100)
	send := notifier.Sender("TestExitBacklog2")

	for i := 1; i <= 50; i++ {
		send("Creating backlog")
	}

	if len(notifier.noteChan) == 0 {
		t.Error("Exit: failed setting upt the test. noteChan is empty")
	}

	go notifier.Run()
	for !notifier.isReady() {
		// wait for logging to start
	}

	notifier.Exit() // wait for backlog to clear, then exit

	if len(notifier.noteChan) > 0 {
		t.Error("Exit: backlog should be cleared before exiting")
	}

}

func TestUnsuportedEndpoint(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	badEndPoint := struct {
		file string
	}{
		file: "mylog.log",
	}

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100, badEndPoint)
	go notifier.Run()
	time.Sleep(50 * time.Millisecond)
	notifier.Exit()
}

func TestNegativeErrorCode(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100)
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

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100)
	send := notifier.Sender("TestUnsupportedValue")

	badValue := struct {
		greet string
	}{
		greet: "Hello, World!",
	}

	send(badValue)
	go notifier.Run()
	for !notifier.isReady() {
		// Wait for notifier to start
	}

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	if err := notifier.Exit(); err != nil {
		t.Error(err.Error())
	}

	w.Close()
	os.Stdout = old
	out := <-outC

	splits := strings.Split(out, "\n")
	if len(splits) != 4 {
		t.Error("Response from io.Pipe does not contain exactly four lines: " + strconv.Itoa(len(splits)))
	}
	response := strings.Split(splits[1], "\t")

	if len(response) != 8 {
		t.Error("log: response does not contain 8 fields: " + strconv.Itoa(len(response)))
	}

	if response[5] != "999" || response[7] != "Unknown value used in notify.send" {
		t.Error("log: bad response to an unsupported value:" + strings.Join(response, " "))
	}

}

func TestJSON(t *testing.T) {

	logfile := os.Getenv("HOME") + "/mytestlog.log"
	defer os.Remove(logfile)

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, true, 100, logfile)
	fail := notifier.Failure("TestJSON")
	fail(0, "Hello, World")

	go notifier.Run()
	for !notifier.isReady() {
		// Wait for notifier to start
	}
	notifier.Exit()

	contents, err := ioutil.ReadFile(logfile)
	if err != nil {
		t.Error("Failed reading mytestlog.log: " + err.Error())
	}

	splits := strings.Split(string(contents), "\n")
	log := logEntry{}
	if errJson := json.Unmarshal([]byte(splits[0]), &log); errJson != nil {
		t.Error("Failed unmarshaling log entry")
	} else {
		if log.Status != "GeneralMessage" {
			t.Errorf("Status mismatch. Expected '%s', got '%s'", "GeneralMessage", log.Status)
		}
	}

}

func TestCodes(t *testing.T) {

	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100)
	fail := notifier.Failure("TestCodes")
	err1 := fail(0, "Hello, World")

	if err1.Error()[0:12] != "Hello, World" {
		t.Error("Wrong message returned by fail: " + err1.Error()[0:12])
	}

	if IsCode(1, err1) {
		t.Error("err1 has code 0, not 1")
	}

	if !IsCode(1, errors.New("Oops")) {
		t.Error("The standard error should be treated as if having code = 1")
	}

}

func TestBadLogRef(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100, "/var/log/syslog")
	notifier2 := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100, os.Getenv("HOME")+"/mytestdir/loggy.log")
	defer os.Remove(os.Getenv("HOME") + "/mytestdir/loggy.log")
	notifier3 := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100, os.Getenv("HOME"))

	if notifier.endpoints.endpointsPtr[0] != os.Stdout {
		t.Error("Bad log reference should be replaced by os.Stdout")
	}

	if notifier2.endpoints.endpointsPtr[0] == nil {
		t.Error("Failed attaching any endpoint")
	}

	if _, err := os.Stat(os.Getenv("HOME") + "/mytestdir/loggy.log"); os.IsNotExist(err) {
		t.Error("Failed creating missing logfile")
	}

	if notifier3.endpoints.endpointsPtr[0] != os.Stdout {
		t.Error("Used an existing directory as logfile. Should use os.Stdout instead")
	}

}

func TestRepeatedEndpoints(t *testing.T) {

	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100, os.Stdout, "mylog", os.Stdout)
	defer os.Remove("mylog")

	if len(notifier.endpoints.endpointsPtr) == 3 {
		os.Stdout = old
		fmt.Println(notifier.endpoints.endpointsPtr)
		t.Error("Failed ignoring duplicate endpoints")
	}

}

func TestNoteToSelf(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100)

	err1 := notifier.noteToSelf("simple message")
	err2 := notifier.noteToSelf(errors.New("error message"))
	err3 := notifier.noteToSelf(newf(999, 1, "error message"))

	if err1 != nil {
		t.Error("noteToSelf should return a string if string given")
	}

	if _, ok := err2.(error); !ok {
		t.Error("noteToSelf should return an error if error given")
	}

	if _, ok := err3.(notification); !ok || err3.(notification).code != 999 {
		t.Error("noteToSelf should return a notification with level 999 if error given")
	}

}

func TestMissingCodes(t *testing.T) {
	old := ignoreStdOut(t)
	defer func() { os.Stdout = old }()

	notifier := NewNotifier("MyService", "MyServiceInstance", true, true, false, 100)
	notifier.notificationCodes = map[int][2]string{
		0: [2]string{"MSG", "GeneralMessage"},
		1: [2]string{"ERR", "GeneralError"},
		3: [2]string{"ERR", "FailedAction"},
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Should panic if sysCodes are missing!")
		}
	}()
	notifier.isOK()

}

func TestEmptyNotifier(t *testing.T) {

	logfile := os.Getenv("HOME") + "/mytestlog2.log"
	defer os.Remove(logfile)

	notifier := NewNotifier("", "", true, true, true, 100, logfile)

	go notifier.Run()
	for !notifier.isReady() {
		// Wait for start
	}

	confirm := make(chan bool)
	go notifier.log(&note{"", "", confirm})
	<-confirm

	go notifier.log(&note{"", newf(1000, 2, "no such code"), confirm})
	<-confirm

	notifier.Exit()

	contents, err := ioutil.ReadFile(logfile)
	if err != nil {
		t.Error("Failed reading mytestlog.log: " + err.Error())
	}

	splits := strings.Split(string(contents), "\n")
	log := logEntry{}
	if errJson := json.Unmarshal([]byte(splits[0]), &log); errJson != nil {
		t.Error("Failed unmarshaling log entry")
	}

	if log.Service != "N/A" {
		t.Errorf("Failed correcting missing service")
	}
	if log.Instance != "N/A" {
		t.Errorf("Failed correcting missing instance")
	}
	if log.Sender != "N/A" {
		t.Errorf("Failed correcting missing sender")
	}
	if log.Level != "MSG" {
		t.Errorf("Failed correcting missing level")
	}
	if log.Code != 0 {
		t.Errorf("Failed correcting missing code")
	}
	if log.Status != "GeneralMessage" {
		t.Errorf("Failed correcting missing status")
	}
	if log.Message != "N/A" {
		t.Errorf("Failed correcting missing message")
	}

	if errJson := json.Unmarshal([]byte(splits[1]), &log); errJson != nil {
		t.Error("Failed unmarshaling log entry")
	}

	if log.Code != 1 {
		t.Error("Failed correcting non-existing code")
	}

}
