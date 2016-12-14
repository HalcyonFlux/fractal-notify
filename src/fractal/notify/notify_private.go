package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type notifier struct {
	service           string            // Service that uses the notifier (e.g. fractal-beacon)
	instance          string            // Unique instance name of the service (e.g. beacon_server_01)
	endpointsPtr      []*os.File        // Slice of endpoints the logger should write to
	logAll            bool              // If true, also logs non-error messages
	noteChan          chan *Note        // Channel the notifier listens on
	notificationCodes map[int][2]string // Map of notification codes and their meanings
	safetySwitch      bool              // Indicator of whether notificationCodes have been changed
}

// syswarn prints a warning without logging it
func syswarn(warn string) {
	fmt.Println("notify:", warn)
}

// openLogFile opens a log file and returns a reference to it
func openLogFile(logfile string) (*os.File, error) {

	// Check validity of file
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
		return os.Stdout, err
	} else {
		return f, nil
	}
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
	Timestamp int    `json:"Timestamp"`
	Service   string `json:"Service"`
	Instance  string `json:"Instance"`
	Sender    string `json:"Sender"`
	Level     string `json:"Level"`
	Code      int    `json:"Code"`
	Status    string `json:"Status"`
	Message   string `json:"Message"`
}

// correct corrects some possible mistakes in logEntry
func (l *logEntry) correct() {

	// No empty strings
	if l.Service == "" {
		l.Service = "N/A"
	}
	if l.Instance == "" {
		l.Instance = "N/A"
	}
	if l.Sender == "" {
		l.Sender = "N/A"
	}
	if l.Level == "" {
		l.Level = "N/A"
	}
	if l.Status == "" {
		l.Status = "N/A"
	}
	if l.Message == "" {
		l.Message = "N/A"
	}

	// No tabs, newlines and so on.
	for _, symbol := range []string{"\t", "\n", "\r", "\b", "\f", "\v"} {
		l.Service = strings.Replace(l.Service, symbol, " ", -1)
		l.Instance = strings.Replace(l.Instance, symbol, " ", -1)
		l.Sender = strings.Replace(l.Sender, symbol, " ", -1)
		l.Level = strings.Replace(l.Level, symbol, " ", -1)
		l.Status = strings.Replace(l.Status, symbol, " ", -1)
		l.Message = strings.Replace(l.Message, symbol, " ", -1)
	}

}

// toStr turns logEntry to string
func (l *logEntry) toStr() string {
	return strconv.Itoa(l.Timestamp) + "\t" + l.Service + "\t" + l.Instance + "\t" + l.Sender + "\t" +
		l.Level + "\t" + strconv.Itoa(l.Code) + "\t" + l.Status + "\t" + l.Message
}

// toJson turns logEntry to json-encoded string
func (l *logEntry) toJson() string {
	jsoned, err := json.Marshal(l)
	if err != nil {
		syswarn("Could not convert logEntry to JSON: " + err.Error())
		return "{\"ERROR\": \"Could not convert logEntry to JSON\"}"
	}

	return string(jsoned)
}

// log logs a message/error
// Log structure (8 fields): Unix-timestamp service_name unique_instance_name sender_name level statusCode statusText Message
//  e.g.:
// 1481552048\tbeacon\tbeacon_server_01\tcollector]\tMSG\t0\tGeneralMessage\tPushing a new Job into the jobChan
// 1481552049\tbeacon\tbeacon_server_01\tdispatcher]\tERR\t3\tSystemMalfunction\tCould not dispatch Job
func (no *notifier) log(note *Note) {

	// Sanity check (will panic)
	no.isOK()

	// Create a new log entry
	lg := logEntry{
		Timestamp: int(time.Now().Unix()),
		Service:   no.service,
		Instance:  no.instance,
		Sender:    note.Sender,
	}

	switch msg := note.Value.(type) {

	case Notification:
		if _, ok := no.notificationCodes[msg.Code]; !ok {
			no.noteToSelf(Newf(999, "Unknown error code used. Replacing '%d' with '1'", msg.Code))
			lg.Code = 1
		} else {
			lg.Code = msg.Code
		}
		lg.Message = msg.Message

	case error:
		lg.Code = 1
		lg.Message = msg.Error()

	case string:
		lg.Code = 0
		lg.Message = msg

	default:
		lg.Code = 999
		lg.Message = "Unknown value used in notify.Send"
	}

	// Determine level and status
	levelStatus, _ := no.notificationCodes[lg.Code]
	lg.Level = levelStatus[0]
	lg.Status = levelStatus[1]

	// Write to all endpoints
	str := lg.toStr()
	for i, w := range no.endpointsPtr {
		if _, werr := w.WriteString(str + "\n"); werr != nil {
			syswarn("failed writing to " + strconv.Itoa(i) + "th endpoint: " + werr.Error()) // do not log to avoid infinite loop
		}
	}

}
