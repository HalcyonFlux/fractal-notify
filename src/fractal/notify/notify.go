package notify

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Notification is the standard error struct used in notify
type Notification struct {
	Code    int
	Message string
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

// New builds a Notification struct
func New(code int, args ...string) error {
	if code < 1 {
		syswarn(fmt.Sprintf("An error should have greater than zero code. Changing %d to 1", code))
		code = 1
	}

	// Append some runtime information
	if _, fn, line, ok := runtime.Caller(1); ok {
		args = append(args, fmt.Sprintf(" -> [%s: %d]", fn, line))
	}

	return Notification{Code: code, Message: strings.Join(args, " ")}
}

// Newf  formats according to a format specifier and runs notify.New
func Newf(code int, format string, a ...interface{}) error {
	return New(code, fmt.Sprintf(format, a))
}

// Error returns the notification text
func (e Notification) Error() string {
	return e.Message
}

// Note is a struct used to transport notifications (string, error, notify.Notification)
// to the note channel.
type Note struct {
	Sender string
	Value  interface{}
}

// NewNotifier instantiates and returns a new notifier and a (send-only) Notes channel.
// Other elements of the system can notify the user/write to log by sending (notify.Send) to the noteChan.
// Accepted endpoints: files.log and os.File implementations (e.g. os.Stdout). Notes will be sent to all
// available endpoints.
// In order to run the logging service, the looper function must be started.
// If blocking behaviour is required, then the looping fuction should be started normally (otherwise as a goroutine).
// Closing the noteChan exits the looper/notifier
func NewNotifier(service string, instance string, logAll bool, notifierCap int, endpoints ...interface{}) (*notifier, chan<- *Note) {

	no := notifier{}

	// Prepare endpoints
	if len(endpoints) == 0 {
		syswarn("No endpoints provided. Going to route all notes to os.Stdout")
		endpoints = []interface{}{os.Stdout}
	}

	for i, endpoint := range endpoints {

		switch w := endpoint.(type) {

		case string:
			f, err := openLogFile(w)
			if err == nil || f == os.Stdout {
				no.endpointsPtr = append(no.endpointsPtr, f)
			}

		case *os.File:
			no.endpointsPtr = append(no.endpointsPtr, w)

		default:
			syswarn(strconv.Itoa(i+1) + "th endpoint is not supported. Either provide a file path (string) or an instance of *os.File")
		}

	}

	// Set agent details
	noteChan := make(chan *Note, notifierCap)
	no.service = service
	no.instance = instance
	no.noteChan = noteChan
	no.logAll = logAll
	no.safetySwitch = false
	no.notificationCodes = standardCodes

	return &no, noteChan
}

// Send creates a Note struct and sends it into the noteChan
func Send(sender string, value interface{}, noteChan chan<- *Note) error {

	go func() {
		noteChan <- &Note{
			Sender: sender,
			Value:  value,
		}
	}()

	if err, ok := value.(error); ok {
		return err
	} else {
		return nil
	}
}

// PersonalizeSend creates a simplified notify.Send function, which requires
// only the value of the message to be passed. Each unique sender (e.g. server,
// client, etc.) should have their own personalized Send, or use the more verbose
// notify.Send.
func (no *notifier) PersonalizeSend(sender string) func(interface{}) error {
	return func(value interface{}) error {
		return Send(sender, value, no.noteChan)
	}
}

// PersonalizeNew creates a simplified notify.Send(notify.New()) function, which requires
// only the value of the error code and message to be passed.
// Each unique sender (e.g. server, client, etc.) should have their own personalized
// New, or use the more verbose notify.Send(notify.New())
func (no *notifier) PersonalizeNew(sender string) func(int, string) error {
	return func(code int, message string) error {
		return Send(sender, New(code, message), no.noteChan)
	}
}

// SetCodes replaces built-in notification codes with custom ones.
// A partial replacement (e.g. codes 2-10) is also allowed, however only one
// call to notifier.SetCodes is allowed per notifier.
func (no *notifier) SetCodes(newCodes map[int][2]string) {

	// Sanity check (will panic)
	no.isOK()

	// Only allow one change of codes per notifier
	if no.safetySwitch {
		fmt.Println()
		no.noteToSelf(New(999, "You are trying to change notification codes again. This action is not permitted."))
		return
	}

	for code, notification := range newCodes {
		if code <= 1 || code >= 999 {
			no.noteToSelf(Newf(999, "Only notification codes 1 < code < 999 are replaceable. Removing '%d'", code))
			delete(newCodes, code)
		} else {
			no.notificationCodes[code] = notification
		}
	}

	no.notificationCodes = newCodes

}

// loop logs and displays messages sent to the Note channel
// Loop is the only consumer of the Note channel as well as the logging facility
func (no *notifier) Loop() {

	// Sanity check
	no.isOK()

	var note *Note
	var ok bool

	// Receive Notes
	for {

		// Receive or quit
		select {
		case note, ok = <-no.noteChan:
			if !ok {
				no.log(&Note{"notifier", "Note channel has been closed. Exiting notifier loop."})
				for _, endpoint := range no.endpointsPtr {
					if endpoint != os.Stdout {
						endpoint.Close()
					}
				}
				return
			}
		}

		// Log (and maybe print)
		switch note.Value.(type) {
		case string:
			if no.logAll {
				no.log(note)
			}
		default:
			no.log(note)
		}
	}

}
