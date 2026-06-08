package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/updater"
)

type UpdateCommand struct{}

func NewUpdateCommand() *UpdateCommand { return &UpdateCommand{} }
func (c *UpdateCommand) Name() string  { return "update" }

func (c *UpdateCommand) Run(args []string) error {
	checkOnly := hasArg(args, "--check")
	force := hasArg(args, "--force")

	info, err := updater.CheckWithTimeout(10 * time.Second)
	if err != nil {
		if !force {
			return fmt.Errorf("controllo aggiornamenti fallito: %w", err)
		}
		fmt.Printf("Controllo aggiornamenti fallito: %s\n", err)
		fmt.Println("Procedo comunque con la reinstallazione forzata.")
	}

	if info.Current != "" {
		fmt.Printf("Versione installata: %s\n", info.Current)
	}
	if info.Latest != "" {
		fmt.Printf("Ultima versione:     %s\n", info.Latest)
	}
	if checkOnly {
		if info.Available {
			fmt.Println("Aggiornamento disponibile. Esegui: vortelio update")
		} else {
			fmt.Println("Vortelio e' gia' aggiornato.")
		}
		return nil
	}
	if !force && !info.Available {
		fmt.Println("Vortelio e' gia' aggiornato. Usa --force per reinstallare.")
		return nil
	}

	res, err := updater.StartDetached(false)
	if err != nil {
		return err
	}
	fmt.Println(res.Message)
	if res.LogPath != "" {
		fmt.Printf("Log: %s\n", res.LogPath)
	}
	return nil
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if strings.EqualFold(arg, want) {
			return true
		}
	}
	return false
}
