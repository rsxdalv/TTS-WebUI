package installer_scripts

import "strings"

// This file ports installer_scripts/js/displayMessage.js. The original also
// copies each message to js/webConsole.js for the status server on port 7771.
// That server is left out (see init_app_nodejs.go), so the copy is too.

// displayMessage ports displayMessage: console.log of the arguments joined by
// spaces.
func (p *jsProcess) displayMessage(message ...string) {
	p.consoleLog(strings.Join(message, " "))
}

// displayError ports displayError: console.error of the arguments joined by
// spaces. The launcher shows the stderr of init_app.js with StderrPrefix, so
// the port logs these lines the same way.
func (p *jsProcess) displayError(message ...string) {
	p.consoleError(strings.Join(message, " "))
}

// consoleLog logs s as console.log prints it: one line per "\n" in s.
func (p *jsProcess) consoleLog(s string) {
	for _, line := range strings.Split(s, "\n") {
		p.s.Log(line)
	}
}

// consoleError is consoleLog for stderr.
func (p *jsProcess) consoleError(s string) {
	for _, line := range strings.Split(s, "\n") {
		p.s.Log(StderrPrefix + line)
	}
}
