package notify

import (
	"fmt"
	"os"
	"path/filepath"
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

// NewNotifier instantiates and returns a new notifier.
// looping function and a (send-only) Notes channel.
// Other elements of the system can notify the user/write to log by sending (notify.Send) to the noteChan.
// In order to run the logging service, the looper function must be started.
// If blocking behaviour is required, then the looping fuction should be started normally (otherwise as a goroutine).
// Closing the noteChan exits the looper
func NewNotifier(service string, instance string, logfile string, verbosity int, notifierCap int) (*notifier, chan<- *Note) {

	no := notifier{}

	// Check if the provided log file is valid
	if strings.ToLower(filepath.Ext(logfile)) != ".log" {
		syswarn("log file's extension is not *.log")
	}

	if f, err := os.Stat(logfile); os.IsNotExist(err) {
		if _, berr := os.Stat(filepath.Dir(logfile)); os.IsNotExist(berr) {
			if derr := os.MkdirAll(filepath.Dir(logfile), 0700); derr != nil {
				syswarn("log file directory does not exist. Failed creating it: " + derr.Error())
			}
		}
	} else if err == nil && f.IsDir() {
		syswarn("the provided log file is a directory. Will not be able to write notifications to file.")
	}

	// Open the log file
	if f, err := os.OpenFile(logfile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err != nil {
		syswarn("Failed opening log file: " + err.Error() + ". Using os.Stdout instead")
		no.logfilePtr = os.Stdout
	} else {
		no.logfilePtr = f
	}

	// Set agent details
	noteChan := make(chan *Note, notifierCap)
	no.service = service
	no.instance = instance
	no.noteChan = noteChan
	no.verbosity = verbosity
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
				no.logfilePtr.Close()
				return
			}
		}

		// Log (and maybe print)
		switch msg := note.Value.(type) {

		case error:
			no.log(note)
			if no.verbosity >= 2 {
				syswarn(fmt.Sprintf("ERR\t %s -> %s\n", note.Sender, msg.Error()))
			}

		case string:
			if no.verbosity >= 1 {
				no.log(note)
			}
			if no.verbosity >= 2 {
				syswarn(fmt.Sprintf("MSG\t %s -> %s\n", note.Sender, msg))
			}

		default:
			if no.verbosity >= 0 {
				no.log(note)
			}
			if no.verbosity >= 2 {
				syswarn("Unknown note value type")
			}
		}

	}
}
