package cli

import (
	"errors"
	"os"

	"github.com/Hyaxia/blogwatcher/internal/config"
)

type printedError struct {
	err error
}

func (e printedError) Error() string {
	return e.err.Error()
}

func markError(err error) error {
	if err == nil {
		return nil
	}
	return printedError{err: err}
}

func isPrinted(err error) bool {
	var printed printedError
	return errors.As(err, &printed)
}

// NotConfiguredError is returned when the app is not initialized.
type NotConfiguredError struct{}

func (e NotConfiguredError) Error() string {
	return "Not configured. Run 'blogwatcher init' first."
}

// RequireConfig checks if configuration exists and is valid.
// Returns NotConfiguredError if config is missing or empty.
func RequireConfig() error {
	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		return err
	}

	// Check if config file exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		return NotConfiguredError{}
	}

	// Check if config has API key
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	if !cfg.IsConfigured() {
		return NotConfiguredError{}
	}

	return nil
}
