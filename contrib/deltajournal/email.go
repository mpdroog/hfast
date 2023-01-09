package main

import (
	"context"
	"deltajournal/config"
	"fmt"
	//"gopkg.in/gomail.v1"
	"github.com/mailgun/mailgun-go/v4"
	"math/rand"
	"time"
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
	// Create an instance of the Mailgun Client
	mg := mailgun.NewMailgun(e.User, e.Pass)
	mg.SetAPIBase("https://api.eu.mailgun.net/v3")

	message := mg.NewMessage(e.From, fmt.Sprintf("[%s] %s", host, e.Subject), body, e.To...)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Send the message with a 10 second timeout
	_, _, err := mg.Send(ctx, message)
	/*if config.Verbose {
		fmt.Printf("ID: %s Resp: %s\n", id, resp)
	}*/
	return err

	/*	msg := gomail.NewMessage()
		msg.SetHeader("Message-ID", fmt.Sprintf("<%s@%s>", RandText(32), host))
		msg.SetHeader("X-Mailer", "deltajournal")
		msg.SetHeader("X-Priority", "3")
		msg.SetHeader("From", e.Display+" <"+e.From+">")
		msg.SetHeader("To", e.To...)
		msg.SetHeader("Subject", fmt.Sprintf("[%s] %s", host, e.Subject))
		msg.SetBody("text/plain", body)

		mailer := gomail.NewMailer(e.Host, e.User, e.Pass, e.Port)
		return mailer.Send(msg)*/
}
