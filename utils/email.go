package utils

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/resend/resend-go/v2"
)

var resendClient *resend.Client

func InitResend() {
	resendClient = resend.NewClient(Cfg.ResendAPIKey)
}

func SendVerificationEmail(email, token string) error {
	verificationURL := fmt.Sprintf("%s/auth/verify?token=%s", Cfg.DevServerBackend, token)

	params := &resend.SendEmailRequest{
		From:    "team@cogneurax.com",
		To:      []string{email},
		Subject: "Verify your email",
		Html: fmt.Sprintf(`
            <h2>Welcome!</h2>
            <p>Click the link below to verify your email:</p>
            <a href="%s">Verify Email</a>
            <p>This link expires in 24 hours.</p>
        `, verificationURL),
	}

	_, err := resendClient.Emails.Send(params)
	return err
}

func AuthRedirect(c *gin.Context, userID string) {
	token, err := CreateAccessToken(userID)
	if err != nil {
		c.AbortWithStatusJSON(500, gin.H{"error": "Failed to create token"})
		return
	}
	SetAuthCookies(c, token, "")
	c.Redirect(302, Cfg.DevServer+"/login/callback")
}
