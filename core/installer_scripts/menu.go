package installer_scripts

import (
	"context"
	"fmt"
)

// menu ports installer_scripts/js/menu.js. The original draws an arrow-key menu
// on the terminal. The port asks through Step.Prompt instead. When Prompt is
// nil nobody can answer, so the port logs why and exits 1 (CONTRACTS.md,
// section 3). It never guesses an answer.
func (p *jsProcess) menu(ctx context.Context, choices []string, label string) (string, error) {
	if p.s.Prompt == nil {
		p.displayError("Cannot ask for a choice: nobody can answer.", label)
		return "", processExitError{1}
	}
	i, err := p.s.Prompt(ctx, label, choices)
	if err != nil {
		return "", err
	}
	if i < 0 || i >= len(choices) {
		return "", fmt.Errorf("menu: choice %d is out of range", i)
	}
	return choices[i], nil
}
