package commands

import (
	"fmt"
	"sort"
	"strings"
)

// Handler is a function that executes a slash command.
// It receives the arguments (everything after the command name).
// Returns a message to display to the user, or an error.
type Handler func(args string) (string, error)

// Command describes a registered slash command.
type Command struct {
	Name        string  // e.g. "/search"
	Description string  // short help text
	Handler     Handler
	Hidden      bool // if true, don't show in /help
}

// Dispatcher manages slash commands.
type Dispatcher struct {
	commands map[string]Command
}

// NewDispatcher creates a dispatcher with no commands registered.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		commands: make(map[string]Command),
	}
}

// Register adds a command. Name must start with "/".
func (d *Dispatcher) Register(cmd Command) error {
	if !strings.HasPrefix(cmd.Name, "/") {
		return fmt.Errorf("command name must start with /: %q", cmd.Name)
	}
	d.commands[cmd.Name] = cmd
	return nil
}

// Execute parses input and runs the matching command.
// Returns (output, handled, error).
// handled is false if the input isn't a slash command.
func (d *Dispatcher) Execute(input string) (string, bool, error) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return "", false, nil
	}

	parts := strings.SplitN(input, " ", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	cmd, ok := d.commands[name]
	if !ok {
		return "", true, fmt.Errorf("unknown command: %s (type /help for available commands)", name)
	}

	result, err := cmd.Handler(args)
	return result, true, err
}

// Help returns a formatted help string listing all visible commands.
func (d *Dispatcher) Help() string {
	var cmds []Command
	for _, c := range d.commands {
		if !c.Hidden {
			cmds = append(cmds, c)
		}
	}
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})

	var b strings.Builder
	b.WriteString("Available commands:\n")
	for _, c := range cmds {
		fmt.Fprintf(&b, "  %-20s %s\n", c.Name, c.Description)
	}
	return b.String()
}

// Commands returns all registered command names.
func (d *Dispatcher) Commands() []string {
	names := make([]string, 0, len(d.commands))
	for n := range d.commands {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
