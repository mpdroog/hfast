package main

import (
	"fmt"
	"gopkg.in/gomail.v1"
	"math/rand"
	"time"
	"deltajournal/config"
)

var (
	letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

// Random string chars, n=length
func RandText(n int) string {
	rand.Seed(time.Now().Unix())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func Email(e config.Email, body string) error {
	msg := gomail.NewMessage()
	msg.SetHeader("Message-ID", fmt.Sprintf("<%s@%s>", RandText(32), host))
	msg.SetHeader("X-Mailer", "deltajournal")
	msg.SetHeader("X-Priority", "3")
	msg.SetHeader("From", e.Display+" <"+e.From+">")
	msg.SetHeader("To", e.To...)
	msg.SetHeader("Subject", e.Subject)
	msg.SetBody("text/plain", body)

	mailer := gomail.NewMailer(e.Host, e.User, e.Pass, e.Port)
	return mailer.Send(msg)
}
