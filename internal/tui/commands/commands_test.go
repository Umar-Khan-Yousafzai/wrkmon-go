package commands

import (
	"strings"
	"testing"
)

func TestRegisterSucceedsWithSlashPrefix(t *testing.T) {
	d := NewDispatcher()
	err := d.Register(Command{
		Name:        "/test",
		Description: "a test command",
		Handler:     func(args string) (string, error) { return "ok", nil },
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRegisterFailsWithoutSlashPrefix(t *testing.T) {
	d := NewDispatcher()
	err := d.Register(Command{
		Name:        "noprefix",
		Description: "missing slash",
		Handler:     func(args string) (string, error) { return "", nil },
	})
	if err == nil {
		t.Fatal("expected error for command without / prefix")
	}
	if !strings.Contains(err.Error(), "must start with /") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExecuteRunsCorrectHandler(t *testing.T) {
	d := NewDispatcher()
	_ = d.Register(Command{
		Name:        "/greet",
		Description: "say hello",
		Handler: func(args string) (string, error) {
			return "hello " + args, nil
		},
	})

	result, handled, err := d.Execute("/greet world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatal("expected handled=true")
	}
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %q", result)
	}
}

func TestExecuteReturnsFalseForNonSlashInput(t *testing.T) {
	d := NewDispatcher()
	_, handled, err := d.Execute("just some text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false for non-slash input")
	}
}

func TestExecuteReturnsErrorForUnknownCommand(t *testing.T) {
	d := NewDispatcher()
	_, handled, err := d.Execute("/unknown")
	if !handled {
		t.Fatal("expected handled=true for slash input")
	}
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestHelpListsVisibleCommands(t *testing.T) {
	d := NewDispatcher()
	_ = d.Register(Command{
		Name:        "/visible",
		Description: "a visible command",
		Handler:     func(args string) (string, error) { return "", nil },
	})
	_ = d.Register(Command{
		Name:        "/hidden",
		Description: "a hidden command",
		Handler:     func(args string) (string, error) { return "", nil },
		Hidden:      true,
	})

	help := d.Help()
	if !strings.Contains(help, "/visible") {
		t.Error("help should contain /visible")
	}
	if strings.Contains(help, "/hidden") {
		t.Error("help should not contain /hidden")
	}
}

func TestCommandsReturnsSortedNames(t *testing.T) {
	d := NewDispatcher()
	_ = d.Register(Command{
		Name:    "/zebra",
		Handler: func(args string) (string, error) { return "", nil },
	})
	_ = d.Register(Command{
		Name:    "/alpha",
		Handler: func(args string) (string, error) { return "", nil },
	})
	_ = d.Register(Command{
		Name:    "/middle",
		Handler: func(args string) (string, error) { return "", nil },
	})

	names := d.Commands()
	if len(names) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(names))
	}
	if names[0] != "/alpha" || names[1] != "/middle" || names[2] != "/zebra" {
		t.Fatalf("expected sorted order [/alpha /middle /zebra], got %v", names)
	}
}
