package handlers

import (
	"fmt"
	"net/mail"
	"strings"

	"Synapse/structs"
)

func normalizeSignup(request *structs.UserCreate) error {
	request.Username = strings.TrimSpace(request.Username)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if len(request.Username) < 3 || len(request.Username) > 32 {
		return fmt.Errorf("username must be between 3 and 32 characters")
	}
	for _, char := range request.Username {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return fmt.Errorf("username may only contain letters, numbers, and underscores")
	}
	if err := validateEmail(request.Email); err != nil {
		return err
	}
	return validatePassword(request.Password)
}

func normalizeLogin(request *structs.UserLogin) error {
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	if err := validateEmail(request.Email); err != nil {
		return err
	}
	return validatePassword(request.Password)
}

func validateEmail(value string) error {
	if len(value) > 254 {
		return fmt.Errorf("invalid email address")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value {
		return fmt.Errorf("invalid email address")
	}
	return nil
}

func validatePassword(value string) error {
	if len(value) < 8 || len(value) > 72 {
		return fmt.Errorf("password must be between 8 and 72 characters")
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("password cannot be blank")
	}
	return nil
}
