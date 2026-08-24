package handlers

import (
	"strings"
	"testing"

	"Synapse/structs"
)

func TestNormalizeSignup(t *testing.T) {
	request := structs.UserCreate{
		Username: "  Dave_123 ",
		Email:    " DAVE123@EXAMPLE.COM ",
		Password: "safe password",
	}
	if err := normalizeSignup(&request); err != nil {
		t.Fatal(err)
	}
	if request.Username != "Dave_123" || request.Email != "dave123@example.com" {
		t.Fatalf("signup was not normalized: %#v", request)
	}
}

func TestNormalizeSignupRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		request structs.UserCreate
	}{
		{name: "short username", request: structs.UserCreate{Username: "ab", Email: "a@example.com", Password: "password"}},
		{name: "punctuation in username", request: structs.UserCreate{Username: "dave!", Email: "a@example.com", Password: "password"}},
		{name: "display-name email", request: structs.UserCreate{Username: "dave", Email: "Dave <a@example.com>", Password: "password"}},
		{name: "blank password", request: structs.UserCreate{Username: "dave", Email: "a@example.com", Password: "        "}},
		{name: "bcrypt overflow", request: structs.UserCreate{Username: "dave", Email: "a@example.com", Password: strings.Repeat("x", 73)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := normalizeSignup(&test.request); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNormalizeLogin(t *testing.T) {
	request := structs.UserLogin{Email: " DAVE@EXAMPLE.COM ", Password: "password"}
	if err := normalizeLogin(&request); err != nil {
		t.Fatal(err)
	}
	if request.Email != "dave@example.com" {
		t.Fatalf("email = %q", request.Email)
	}
}
