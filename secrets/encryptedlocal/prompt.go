package encryptedlocal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode"

	"golang.org/x/term"
)

var (
	ErrInvalidPassword = errors.New(
		"password must contain at least one number, one uppercase letter, one special character," +
			" and be at least 8 characters long",
	)
	ErrPasswordMismatch    = errors.New("passwords do not match")
	ErrTerminatedOperation = errors.New("operation terminated")
)

type Prompt struct {
}

func NewPrompt() *Prompt {
	return &Prompt{}
}

func (p *Prompt) GeneratePassword() ([]byte, error) {
	bytePassword, err := p.InputPassword(true)
	if err != nil {
		return nil, err
	}

	_, err = p.ConfirmPassword(bytePassword)
	if err != nil {
		return nil, err
	}

	return bytePassword, nil
}

type MnemonicGenerator interface {
	GenerateMnemonic() (string, error)
}

func (p *Prompt) GenerateMnemonic(generator MnemonicGenerator) (string, error) {
	fmt.Println("\nWe must generate a mnemonic which will represent your node account.")
	fmt.Println("\nKeep it safe and do not share it with anyone.")
	fmt.Println("\nYou will need this mnemonic to recover your node account if you lose it.")
	start, err := p.DefaultPrompt("Do you want to start the process? [y/n]", "y")
	if err != nil {
		return "", err
	}

	start = strings.ToLower(start)
	if start != "y" {
		return "", ErrTerminatedOperation
	}

	mnemonic, err := generator.GenerateMnemonic()
	if err != nil {
		return "", err
	}

	fmt.Println("\nHere is your mnemonic. Please copy it and store it in a safe place.")
	repeatMnemonic, err := p.DefaultPrompt(
		"Please re-type the mnemonic to confirm that you have backed it up in a safe location.",
		"",
	)
	if err != nil {
		return "", err
	}

	if repeatMnemonic != mnemonic {
		return "", errors.New("mnemonic mismatch")
	}

	if err != nil {
		return "", err
	}

	return mnemonic, nil
}

func (p *Prompt) InputPassword(verify bool) ([]byte, error) {
	return p.promptUntil(func() ([]byte, error) {
		if verify {
			fmt.Println("\nYou must generate a password to encrypt your secrets.")
		} else {
			fmt.Println("\nYou must enter your password to decrypt your secrets.")
		}

		fmt.Print("Enter password: ")
		bytePassword, err := p.readPassword()
		if err != nil {
			return nil, err
		}

		password := string(bytePassword)
		if verify && !verifyPassword(password) {
			return nil, ErrInvalidPassword
		}

		return bytePassword, nil
	})
}

func (p *Prompt) ConfirmPassword(password []byte) (pass []byte, err error) {
	return p.promptUntil(func() ([]byte, error) {
		fmt.Print("\nConfirm password: ")
		rePassword, err := p.readPassword()
		if err != nil {
			return nil, fmt.Errorf("error reading password: %w", err)
		}

		if !bytes.Equal(password, rePassword) {
			return nil, ErrPasswordMismatch
		}

		return pass, err
	})
}

func (p *Prompt) readPassword() ([]byte, error) {
	return term.ReadPassword(syscall.Stdin)
}

// DefaultPrompt prompts the user for any text and performs no validation. If nothing is entered it returns the default.
func (p *Prompt) DefaultPrompt(promptText, defaultValue string) (string, error) {
	var response string
	if defaultValue != "" {
		fmt.Printf("%s %s:\n", promptText, fmt.Sprintf("(default: %s)", defaultValue))
	} else {
		fmt.Printf("%s:\n", promptText)
	}

	scanner := bufio.NewScanner(os.Stdin)
	if ok := scanner.Scan(); ok {
		item := scanner.Text()
		response = strings.TrimRight(item, "\r\n")
		if response == "" {
			return defaultValue, nil
		}

		return response, nil
	}

	return "", errors.New("could not scan text input")
}

func verifyPassword(s string) (ok bool) {
	var (
		number  bool
		upper   bool
		special bool
	)

	for _, c := range s {
		switch {
		case unicode.IsNumber(c):
			number = true
		case unicode.IsUpper(c):
			upper = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			special = true
		}
	}

	eightOrMore := len(s) > 7

	return number && upper && special && eightOrMore
}

func (p *Prompt) promptUntil(action func() (res []byte, err error)) (res []byte, err error) {
	for {
		res, err := action()
		if err != nil {
			fmt.Printf("\n %s \n", err)
			agree, err := p.tryAgain()
			if err != nil {
				return nil, err
			}

			if !agree {
				return nil, ErrTerminatedOperation
			}

			continue
		}

		return res, nil
	}
}

func (p *Prompt) tryAgain() (agree bool, err error) {
	res, err := p.DefaultPrompt("Try again? [y/n]", "y")
	if err != nil {
		return false, err
	}

	res = strings.ToLower(res)
	switch res {
	case "y":
		return true, nil
	case "n":
		return false, nil
	default:
		fmt.Println("Invalid input")

		return p.tryAgain()
	}
}
