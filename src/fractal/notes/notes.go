package notes

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Error codes
var statusCodes = map[int]string{
	//0: "NormalMessage",
	1:   "GeneralError",
	2:   "ConfigurationError",
	3:   "SystemMalfunction",
	4:   "UserError",
	5:   "HTTPError",	
	999: "ProgrammingMistake",
}

// ERR_BEACON is the standard error struct used in fractal/beacon
type ErrFractal struct {
	StatusCode    int
	StatusMessage string
	Message       string
}

// New builds an ErrFractal error struct
func New(code int, args ...string) error {

	// Check if error code is valid
	statusMessage, ok := statusCodes[code]
	if !ok {
		panic("Unknown ErrFractal error code: " + strconv.Itoa(code))
	}

	// Build error struvct
	message := ""
	if len(args) == 1 {
		message = args[0]
	} else {
		for _, s := range args {
			message += " " + s
		}
	}

	err := ErrFractal{code, statusMessage, message}

	return err
}

// Error returns the verbose error message
func (e ErrFractal) Error() string {
	return fmt.Sprintf("\t%d\t%s\t%s", e.StatusCode, e.StatusMessage, e.Message)
}

type Note struct {
	Value  interface{}
	Sender string
}

// Notify creates a Note struct and sends it into the noteChan
func Notify(sender string, value interface{}, noteChan chan<- *Note) error {

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

// NewNotifier creates a shortcut for sendInfo
func NewNotifier(sender string, noteChan chan<- *Note) func(value interface{}) error {
	return func(value interface{}) error {
		return Notify(sender, value, noteChan)
	}
}

type Logger struct {
	service      string       // Service that uses the logger (e.g. fractal-beacon)
	instanceName string       // Unique instance name of the service (e.g. beacon_server_01)
	logfilePtr   *os.File     // Full path to the log file
	verbosity    int          // Level of output (0 - only log errors, 1 - log errors and messages, 2 - log and display both)
	noteChan     <-chan *Note // Channel the logger listens on
}

// NewLogger instantiates a new logger and returns a looping function and a (send-only) Notes channel.
// Other elements of the system can notify the user/write to log via the noteChan.
// In order to run the logging service, the looper function must be started.
// If blocking behaviour is expected, then the looping fuction should be started normally (otherwise run a go routine).
// Closing the noteChan exits the loop
func NewLogger(service string, instance string, logfile string, verbosity int, loggerCap int) (func(), chan<- *Note) {

	logger := Logger{}

	// Open log file
	if _, err := os.Stat(logfile); logfile != "" && os.IsNotExist(err) {
		fmt.Println("Log file does not exist! Using os.Stdout instead!")
		logger.logfilePtr = os.Stdout
	} else {
		if f, err := os.Open(logfile); err != nil {
			fmt.Println("Failed opening log file: " + err.Error() + ". Using os.Stdout instead!")
			logger.logfilePtr = os.Stdout
		} else {
			logger.logfilePtr = f
		}
	}

	noteChan := make(chan *Note, loggerCap)
	logger.service = service
	logger.instanceName = instance
	logger.noteChan = noteChan
	logger.verbosity = verbosity

	return logger.loop, noteChan
}

// log logs a message/error and, if necessary, sends a distress signal
// Log structure (8 fields): Unix-timestamp service_name unique_instance_name sender_name level statusCode statusText Message
//  e.g.:
// 1481552048\tbeacon\tbeacon_server_01\tcollector]\tMSG\t0\tNormalMessage\tPushing a new Job into the jobChan
// 1481552049\tbeacon\tbeacon_server_01\tdispatcher]\tERR\t3\tSystemMalfunction\tCould not dispatch Job
func (logger *Logger) log(note *Note) {

	// Log source
	who := logger.service + "\t" + logger.instanceName + "\t" + note.Sender

	// Format message
	msg := ""
	suffix := strconv.Itoa(int(time.Now().Unix())) + "\t" + who
	if err, ok := note.Value.(ErrFractal); ok {
		msg = suffix + "\tERR\t" + err.Error()
	}
	if str, ok := note.Value.(string); ok {
		msg = suffix + "\tMSG\t" + "0\tNormalMessage\t" + str
	}

	// Write to file
	if _, werr := logger.logfilePtr.WriteString(msg); werr != nil {
		fmt.Println("LOG-ERROR: failed writing to log file: " + werr.Error())
	}

}

// loop logs and displays messages sent to the Note channel
// Loop is the only consumer of the Note channel as well as the logging facility
func (logger *Logger) loop() {

	var note *Note
	var ok bool

	// Receive notes and errors
	for {

		// Receive notes or quit
		select {
		case note, ok = <-logger.noteChan:
			if !ok {
				logger.logfilePtr.Close()
				logger.log(&Note{"Logger", "Note channel has been closed. Exiting logger loop."})
				return
			}
		}

		// Anonymous sender
		if note.Sender == "" {
			note.Sender = "N/A"
		}

		// Note type
		noteType := "error"
		if _, okStr := note.Value.(string); okStr {
			noteType = "note"
		}

		// Log (and print)
		switch noteType {

		case "error":
			logger.log(note)
			if logger.verbosity > 0 {
				fmt.Printf("ERR\t %s -> %s\n", note.Sender, note.Value.(error).Error())
			}

		default:
			if logger.verbosity > 0 {
				logger.log(note)
			}
			if logger.verbosity > 1 {
				fmt.Printf("MSG\t %s -> %s\n", note.Sender, note.Value.(error).Error())
			}
		}

	}
}
