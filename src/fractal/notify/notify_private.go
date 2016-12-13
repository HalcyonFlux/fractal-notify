package notify

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type notifier struct {
	service           string            // Service that uses the notifier (e.g. fractal-beacon)
	instance          string            // Unique instance name of the service (e.g. beacon_server_01)
	logfilePtr        *os.File          // Full path to the log file
	verbosity         int               // Level of output (0 - only log errors, 1 - log errors and messages, 2 - log and display both)
	noteChan          chan *Note        // Channel the notifier listens on
	notificationCodes map[int][2]string // Map of notification codes and their meanings
	safetySwitch      bool              // Indicator of whether notificationCodes have been changed
}

// syswarn prints a warning without logging it
func syswarn(warn string) {
	fmt.Println("notify:", warn)
}

// noteToSelf creates a note. This function is used to communicate internal problems.
// This note will be logged
func (no *notifier) noteToSelf(value interface{}) {
	no.noteChan <- &Note{"notifier", value}
}

// isOK check is some assumptions made by the notifier are still valid
// notify.notifier expects some notification codes as well as the notes channel
// to be available at all times.
func (no *notifier) isOK() {

	// Check cannel
	if no.noteChan == nil {
		panic("[notify]: note channel is not available!")
	}

	// Check codes
	sysCodes := []int{0, 1, 999}
	for _, code := range sysCodes {
		if _, okStd := no.notificationCodes[code]; !okStd {
			panic(fmt.Sprintf("notify: notificationCodes[%d] is not available", code))
		}
	}
}

type logEntry struct {
	timestamp int
	service   string
	instance  string
	sender    string
	level     string
	code      int
	status    string
	message   string
}

// correct corrects some possible mistakes in logEntry
func (l *logEntry) correct() {

	// No empty strings
	if l.service == "" {
		l.service = "N/A"
	}
	if l.instance == "" {
		l.instance = "N/A"
	}
	if l.sender == "" {
		l.sender = "N/A"
	}
	if l.level == "" {
		l.level = "N/A"
	}
	if l.status == "" {
		l.status = "N/A"
	}
	if l.message == "" {
		l.message = "N/A"
	}

	// No tabs, newlines and so on.
	for _, symbol := range []string{"\t", "\n", "\r", "\b", "\f", "\v"} {
		l.service = strings.Replace(l.service, symbol, " ", -1)
		l.instance = strings.Replace(l.instance, symbol, " ", -1)
		l.sender = strings.Replace(l.sender, symbol, " ", -1)
		l.level = strings.Replace(l.level, symbol, " ", -1)
		l.status = strings.Replace(l.status, symbol, " ", -1)
		l.message = strings.Replace(l.message, symbol, " ", -1)
	}

}

// toStr turns logEntry to string
func (l *logEntry) toStr() string {
	return strconv.Itoa(l.timestamp) + "\t" + l.service + "\t" + l.instance + "\t" + l.sender + "\t" +
		l.level + "\t" + strconv.Itoa(l.code) + "\t" + l.status + "\t" + l.message
}

// log logs a message/error
// Log structure (8 fields): Unix-timestamp service_name unique_instance_name sender_name level statusCode statusText Message
//  e.g.:
// 1481552048\tbeacon\tbeacon_server_01\tcollector]\tMSG\t0\tGeneralMessage\tPushing a new Job into the jobChan
// 1481552049\tbeacon\tbeacon_server_01\tdispatcher]\tERR\t3\tSystemMalfunction\tCould not dispatch Job
func (no *notifier) log(note *Note) {

	// Sanity check (will panic)
	no.isOK()

	// New log entry
	lg := logEntry{
		timestamp: int(time.Now().Unix()),
		service:   no.service,
		instance:  no.instance,
		sender:    note.Sender,
	}

	switch msg := note.Value.(type) {

	case Notification:
		if _, ok := no.notificationCodes[msg.Code]; !ok {
			no.noteToSelf(Newf(999, "Unknown error code used. Replacing '%d' with '1'", msg.Code))
			lg.code = 1
		} else {
			lg.code = msg.Code
		}
		lg.message = msg.Message

	case error:
		lg.code = 1
		lg.message = msg.Error()

	case string:
		lg.code = 0
		lg.message = msg

	default:
		lg.code = 999
		lg.message = "Unknown value used in notify.Send"
	}

	// Determine level and status
	levelStatus, _ := no.notificationCodes[lg.code]
	lg.level = levelStatus[0]
	lg.status = levelStatus[1]

	lg.correct()

	// Write to file
	if _, werr := no.logfilePtr.WriteString(lg.toStr() + "\n"); werr != nil {
		syswarn("failed writing to log file: " + werr.Error()) // do not log to avoid infinite loop
	}

}
