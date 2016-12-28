// Package notify is a simplistic notification/logging service used in HalcyonFlux/fractal.
//
// Use a function created by notifier.Sender or notifier.Failure to send and
// log notifications. notify.Sender understands string, notify.notification types
// and the errors.error interface.
//
// Use notifier.Run() either sequentially or in a goroutine to run the service.
//
// General advices on using notify:
// * even though you can have several notifiers pointing to different logfiles,
// use as little notifiers as possible to simplify log-analysis.
//
// * use notifier.Sender and notifier.Failure to create personalized notification
// senders.
//
// * use os.Stdout as an endpoint to view logs and messages in debug mode.
//
// * connect notify to remote services by suplying an instance of *os.File (e.g.
// a write-end of os.Pipe. Then use the read-end to get notifications).
package notify
