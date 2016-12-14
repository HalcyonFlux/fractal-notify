package notify

import (
	"os"
	"strconv"
)

// Notification is the standard error struct used in notify
type Notification struct {
	Code    int
	Message string
}

// Error returns the notification text
func (e Notification) Error() string {
	return e.Message
}

// IsCode checks whether the provided error has the error code %code%.
// errors.error implementations that are not notify.Notification are going to be
// given code = 1.
func IsCode(code int, err error) bool {
	if _, ok := err.(Notification); ok {
		return code == err.(Notification).Code
	} else {
		return code == 1
	}
}

// NewNotifier instantiates and returns a new notifier and a (send-only) Notes channel.
// Other elements of the system can notify the user/write to log by sending (notify.Send) to the noteChan.
// Accepted endpoints: files.log and os.File implementations (e.g. os.Stdout). Notes will be sent to all
// available endpoints.
// In order to run the logging service, the looper function must be started.
// If blocking behaviour is required, then the looping fuction should be started normally (otherwise as a goroutine).
// Closing the noteChan exits the looper/notifier
func NewNotifier(service string, instance string, logAll bool, notifierCap int, files ...interface{}) *notifier {

	// Initialize bare notifier
	no := notifier{}

	// Prepare endpoints
	if len(files) == 0 {
		syswarn("No endpoints provided. Going to route all notes to os.Stdout")
		files = []interface{}{os.Stdout}
	}

	for i, endpoint := range files {

		switch w := endpoint.(type) {

		case string:
			f, err := openLogFile(w)
			if err == nil || f == os.Stdout {
				no.endpoints.endpointsPtr = append(no.endpoints.endpointsPtr, f)
			}

		case *os.File:
			no.endpoints.endpointsPtr = append(no.endpoints.endpointsPtr, w)

		default:
			syswarn(strconv.Itoa(i+1) + "th endpoint is not supported. Either provide a file path (string) or an instance of *os.File")
		}

	}

	// Set agent details
	noteChan := make(chan *note, notifierCap)
	no.service = service
	no.instance = instance
	no.noteChan = noteChan
	no.logAll = logAll
	no.safetySwitch = false
	no.notificationCodes = standardCodes

	return &no
}

// Sender creates a simplified notify.send function, which requires
// only the value of the message to be passed. Each unique sender (e.g. server,
// client, etc.) should have their own personalized send.
func (no *notifier) Sender(sender string) func(interface{}) error {
	return func(value interface{}) error {
		return send(sender, value, no.noteChan, &no.ops)
	}
}

// Failure creates a simplified notify.Send(notify.New()) function, which requires
// only the value of the error code and message to be passed.
// Each unique sender (e.g. server, client, etc.) should have their own
// personalized new and/or send.
func (no *notifier) Failure(sender string) func(int, string, ...interface{}) error {
	return func(code int, format string, a ...interface{}) error {
		return send(sender, newf(code, format, a...), no.noteChan, &no.ops)
	}
}

// SetCodes replaces built-in notification codes with custom ones.
// A partial replacement (e.g. codes 2-10) is also allowed, however only one
// call to notifier.SetCodes is allowed per notifier.
func (no *notifier) SetCodes(newCodes map[int][2]string) error {

	// Sanity check (will panic)
	no.isOK()

	// Fail counter
	fails := 0

	// Only allow one change of codes per notifier
	if no.safetySwitch {
		return no.noteToSelf(newf(999, "You are trying to change notification codes again. This action is not permitted."))
	}

	for code, notification := range newCodes {
		if code <= 1 || code >= 999 {
			no.noteToSelf(newf(4, "Only notification codes 1 < code < 999 are replaceable. Removing '%d'", code))
			delete(newCodes, code)
			fails++
		} else {
			no.notificationCodes[code] = notification
		}
	}

	if fails > 0 {
		return newf(4, "Failed replacing %d status codes: invalid range", fails)
	} else {
		return nil
	}

}

// Run logs and displays messages sent to the Note channel
// Run is the only consumer of the Note channel as well as the logging facility
func (no *notifier) Run() {

	// Sanity check
	no.isOK()

	no.endpoints.Lock()

	var n *note
	var ok bool

	// Receive Notes
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
		switch (*n.Value).(type) {
		case string:
			if no.logAll {
				no.log(n)
			}
		default:
			no.log(n)
		}

	}
}

// Exit closes the note channel and waits a little for the notifier to
func (no *notifier) Exit() {

	// Notify about exiting. Might block until space in the channel is available
	var exitMSG interface{} = "Note channel has been closed. Exiting notifier loop."
	no.noteChan <- &note{"notifier", &exitMSG}

	// Close channel and
	no.ops.Lock()
	no.ops.halt = true
	close(no.noteChan)
	no.ops.Unlock()

	// wait for the backlog to be written
	for len(no.noteChan) > 0 {
		// do nothing
	}

	// Close endpoints
	no.endpoints.Lock()
	for _, endpoint := range no.endpoints.endpointsPtr {
		if endpoint != os.Stdout {
			endpoint.Close()
		}
	}
	no.endpoints.Unlock()

}
