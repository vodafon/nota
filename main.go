package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log"

	"gopkg.in/gomail.v2"
)

var (
	flagSubject = flag.String("s", "", "subject")
)

type Config struct {
	Type    string `json:"type"`
	Webhook string `json:"webhook"`
	Email   Email  `json:"email"`
}

type Email struct {
	From     string `json:"from"`
	Password string `json:"password"`
	To       string `json:"to"`
}

type Processor struct {
	cfg     Config
	subject string
	r       io.Reader
}

func main() {
	flag.Parse()

	cfg := mustLoadConfig()

	r := io.TeeReader(os.Stdin, os.Stdout)

	proc := &Processor{
		cfg:     cfg,
		subject: *flagSubject,
		r:       r,
	}

	if proc.subject == "" {
		proc.subject = "Nota"
	}

	switch cfg.Type {
	case "gmail":
		proc.gmailProcess()
	case "discord":
		proc.discordWH()
	default:
		log.Fatalf("invalid type %q", cfg.Type)
	}
}

type DiscordBody struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func (obj *Processor) discordWH() {
	res, err := ioutil.ReadAll(obj.r)
	if err != nil {
		log.Fatal(err)
	}

	resStr := strings.TrimSpace(string(res))
	if len(resStr) >= 2000 {
		resStr = fmt.Sprintf("%s\n... NOT FULL MESSAGE", resStr[:1700])
	}

	if len(resStr) == 0 {
		resStr = "DONE w/o results"
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	jsonValue, _ := json.Marshal(DiscordBody{
		Username: obj.subject,
		Content:  resStr,
	})
	_, err = client.Post(obj.cfg.Webhook, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Fatal(err)
	}
}

func (obj *Processor) gmailProcess() {
	res, err := ioutil.ReadAll(obj.r)
	if err != nil {
		log.Fatal(err)
	}

	d := gomail.NewDialer("smtp.gmail.com", 587, obj.cfg.Email.From, obj.cfg.Email.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	m := gomail.NewMessage()

	m.SetHeader("From", obj.cfg.Email.From)
	m.SetHeader("To", obj.cfg.Email.To)
	m.SetHeader("Subject", obj.subject)

	// Set E-Mail body. You can set plain text or html with text/html
	m.SetBody("text/plain", string(res))

	err = d.DialAndSend(m)
	if err != nil {
		log.Printf("send email error: %v", err)
	}
}

func mustLoadConfig() Config {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("find homedir error: %v", err)
	}

	path := filepath.Join(homedir, ".nota.json")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("read config from %q error: %v", path, err)
	}

	cfg := Config{}
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		log.Fatalf("Unmarshal config file %q error: %v", path, err)
	}
	return cfg
}
