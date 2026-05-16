package commands

// Command is the interface every Vortelio subcommand must implement.
type Command interface {
	Name() string
	Run(args []string) error
}
