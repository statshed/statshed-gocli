package cli

import (
	"log/syslog"

	"github.com/statshed/statshed-cli/internal/config"
)

// AIDEV-NOTE: Syslog is used when submit.syslog is enabled, primarily for
// daemon/cron scenarios where stderr may not be monitored. Facility names map
// to the standard syslog facilities; unknown names fall back to "user".
var syslogFacilities = map[string]syslog.Priority{
	"user":   syslog.LOG_USER,
	"daemon": syslog.LOG_DAEMON,
	"local0": syslog.LOG_LOCAL0,
	"local1": syslog.LOG_LOCAL1,
	"local2": syslog.LOG_LOCAL2,
	"local3": syslog.LOG_LOCAL3,
	"local4": syslog.LOG_LOCAL4,
	"local5": syslog.LOG_LOCAL5,
	"local6": syslog.LOG_LOCAL6,
	"local7": syslog.LOG_LOCAL7,
}

// logToSyslog writes a warning-level message to syslog under the given facility.
func logToSyslog(message, facility string) {
	prio, ok := syslogFacilities[facility]
	if !ok {
		prio = syslog.LOG_USER
	}
	w, err := syslog.New(syslog.LOG_WARNING|prio, "statshed")
	if err != nil {
		return
	}
	defer w.Close()
	_ = w.Warning(message)
}

// logSubmitError logs a submit error to syslog when enabled in config.
// When syslog is disabled this is a no-op and the caller handles stderr output.
func logSubmitError(err error, submit config.SubmitConfig) {
	if submit.Syslog {
		logToSyslog(err.Error(), submit.SyslogFacility)
	}
}
