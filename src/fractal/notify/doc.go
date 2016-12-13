// Package notify is a simplistic notification/logging service used in HalcyonFlux/fractal.
//
// Use notify.Send, or a function created by notifier.PersonalizeSend, to send
// and log notifications.
// notify.Send understands string, notify.Notification types and the errors.error
// interface.
//
// Use notifier.Loop() either sequentially or in a goroutine to run the service.
//
// General advices on using notify:
// * even though you can have several notifiers pointing to different logfiles,
// use as little notifiers as possible to simplify log-analysis.
//
// * use notifier.PersonalizeSend to simplify notifications
//
// * use notify.Send (or a personalized function) in case you want to both
// display/log the error and return an object implementing errors.error interface.
package notify
