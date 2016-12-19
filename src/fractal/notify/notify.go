package notify

import (
	"errors"
	"os"
	"strconv"
)

type Notifier struct {
	service           string            // Service that uses the notifier (e.g. fractal-beacon)
	instance          string            // Unique instance name of the service (e.g. beacon_server_01)
	logAll            bool              // If true, also logs non-error messages
	noteChan          chan *note        // Channel the notifier listens on
	notificationCodes map[int][2]string // Map of notification codes and their meanings
	async             bool              // Indicator of whether notify.send should start goroutines or potentially block
	json              bool              // Indicator of whether logs should be written as json (each line a json object)
	ops               operations        // Lockable operations indicator
	endpoints         endpoints         // Lockable slice of resources
}

// Error returns the notification text
func (e notification) Error() string {
	return e.message
}

// IsCode checks whether the provided error has the error code %code%.
// errors.error implementations that are not notify.notification are going are
// treated as if having code=1.
func IsCode(code int, err error) bool {
	if _, ok := err.(notification); ok {
		return code == err.(notification).code
	} else {
		return code == 1
	}
}

// NewNotifier instantiates and returns a new notifier instance (notifier).
// The notification service is started by running notifier.Run()
// If blocking behaviour is required, then Run() should be started normally
// (otherwise as a goroutine). The notification service is stopped by running
// notifier.Exit(). This command will also exit a blocking Run().
//
// Accepted endpoints: string referenes to files (e.g. myservice.log) and
// pointers to implementations of the os.File interface type (e.g. os.Stdout).
// Notes will be sent to all defined endpoints in their specified order.
//
// Other elements of the system can notify the user/write to log by creating and
// using send and fail functions (created by notifier.Sender, notifier.Failure).
// By default these commands will block if the defined capacity of the notes
// channel is too low for the notification stream. The notifier can be made
// non-blocking by instantiating it with async=true. This will make all send
// and fail commands non-blocking, but the order of log entries cannot be
// guaranteed, i.e. issuing two sends sequentially can result in reversed log entry
// order. It is thus best to set a higher capacity of the notes channel at instantiation.
func NewNotifier(service string, instance string, logAll bool, async bool, json bool, notifierCap int, files ...interface{}) *Notifier {

	// Initialize a bare notifier
	no := Notifier{}

	// Prepare endpoints
	if len(files) == 0 {
		syswarn("No endpoints provided. Going to route all notes to os.Stdout")
		files = []interface{}{os.Stdout}
	}

	endpointSlice := []*os.File{}
	for i, endpoint := range files {

	endSwitch:
		switch w := endpoint.(type) {

		case string:
			f, err := openLogFile(w)

			// disallow writing to the same file
			if err == nil && f != os.Stdout {
				for _, e := range usedFileEndpoints {
					if e == w {
						syswarn("File endpoint " + w + " is already used by another notifier!")
						f.Close()
						break endSwitch
					}
				}
				usedFileEndpoints = append(usedFileEndpoints, w)
			}

			if err == nil || f == os.Stdout {
				endpointSlice = append(endpointSlice, f)
			}

		case *os.File:
			endpointSlice = append(endpointSlice, w)

		default:
			syswarn(strconv.Itoa(i+1) + "th endpoint is not supported. Either provide a file path (string) or an instance of *os.File")
		}

	}

	// Remove duplicates
	dups := make(map[*os.File]bool)
	for _, fi := range endpointSlice {
		if _, ok := dups[fi]; !ok {
			dups[fi] = true
			no.endpoints.endpointsPtr = append(no.endpoints.endpointsPtr, fi)
		}
	}

	// Set agent details
	noteChan := make(chan *note, notifierCap)
	no.service = service
	no.instance = instance
	no.noteChan = noteChan
	no.logAll = logAll
	no.notificationCodes = standardCodes
	no.async = async
	no.json = json
	no.ops.halt = false
	no.ops.running = false

	return &no
}

// Sender creates a simplified notify.send function, which requires
// only the value of the message to be passed. Each unique sender (e.g. server,
// client, etc.) should have their own personalized send.
func (no *Notifier) Sender(sender string) func(interface{}) error {
	return func(value interface{}) error {
		var err error

		// Avoid double sends
		if _, ok := value.(notification); !ok {
			err = send(sender, value, nil, no.noteChan, no.async, &no.ops)
		}

		return err
	}
}

// Failure creates a simplified notify.Send(notify.Newf()) function, which requires
// only the value of the error code and message to be passed.
// Each unique sender (e.g. server, client, etc.) should have their own
// personalized new and/or send functions.
func (no *Notifier) Failure(sender string) func(int, string, ...interface{}) error {
	return func(code int, format string, a ...interface{}) error {
		err := send(sender, newf(code, 2, format, a...), nil, no.noteChan, no.async, &no.ops)
		return err
	}
}

// SetCodes replaces built-in notification codes with custom ones.
// A partial replacement (e.g. codes 2-10) is also allowed, however only calls
// to notifier.SetCodes of a non-active notifier are allowed. After executing
// notifier.Run() a change of codes is not permited anymore.
func (no *Notifier) SetCodes(newCodes map[int][2]string) error {

	// Sanity check (will panic)
	no.isOK()

	if no.isReady() {
		return newf(4, 1, "Cannot change codes on a running notifier")
	}

	// Change codes
	fails := 0
	for code, notification := range newCodes {
		if code <= 1 || code >= 999 {
			no.noteToSelf(newf(4, 1, "Only notification codes 1 < code < 999 are replaceable. Removing '%d'", code))
			delete(newCodes, code)
			fails++
		} else {
			no.notificationCodes[code] = notification
		}
	}

	if fails > 0 {
		return newf(4, 1, "Failed replacing %d status codes: invalid range", fails)
	} else {
		return nil
	}
}

// Run logs messages sent to the note channel
// Run is the only consumer of the note channel as well as the logging facility
func (no *Notifier) Run() {

	// Sanity check
	no.isOK()

	// Enable operations
	no.ops.Lock()
	no.ops.halt = false
	no.ops.running = true
	no.ops.Unlock()

	// Receive Notes
	var n *note
	var ok bool

	no.endpoints.Lock()
runLoop:
	for {

		// Receive or halt notifier operations (send and newf won't work)
		select {
		case n, ok = <-no.noteChan:
			if !ok {
				no.endpoints.Unlock()
				break runLoop
			}
		}

		// Write to endpoints
		switch (n.Value).(type) {
		case string:
			if no.logAll {
				no.log(n)
			}
		default:
			no.log(n)
		}

	}
}

// WarmUp waits until the notifier is ready.
// This function is relevant only when notifier.Run() is started as a goroutine.
func (no *Notifier) WarmUp() {
	for !no.isReady() {
		// Wait for the notifier to start
	}
}

// Exit closes the note channel and waits a little for the notifier to finish logging
func (no *Notifier) Exit() error {

	var err error

	running := true
	if !no.isReady() {
		running = false
		err = errors.New(no.id() + " was not running at exit time.")
	}

	// Halt operations and issue last log entry
	if running {
		no.ops.Lock()
		no.ops.halt = true
		confirm := make(chan bool)
		no.noteChan <- &note{"notifier", "Exit() command has been executed. Stopping the notification service.", confirm}
		no.ops.Unlock()

		<-confirm
		close(confirm)
		close(no.noteChan)
	}

	// Close endpoints
	no.endpoints.Lock()
	for _, endpoint := range no.endpoints.endpointsPtr {
		if endpoint != os.Stdout {
			endpoint.Close()
		}
	}
	no.endpoints.Unlock()

	// Set status
	no.ops.Lock()
	no.ops.running = false
	no.ops.Unlock()

	return err
}
