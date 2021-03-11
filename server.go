package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/mailway-app/config"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/mhale/smtpd"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// https://www.iana.org/assignments/smtp-enhanced-status-codes/smtp-enhanced-status-codes.xhtml
var (
	internalError = errors.New("451 4.3.0 Internal server errror")
	httpClient    *retryablehttp.Client

	seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))
)

const (
	// prefix in lowercase to normalize and match against incoming headers
	INT_HEADER_PREFIX = "mw-int-"

	MW_WEBHOOK_SECRET_TOKEN = "Mw-Int-Webhook-Secret-Token"
	MW_WEBHOOK_URL          = "Mw-Int-Webhook-Url"

	MW_BODY_SECRET_TOKEN = "Mw-Int-Maildb-Secret-Token"
	CRLF                 = "\r\n"
)

type WebhookPayload struct {
	Headers mail.Header `json:"headers"`
	BodyURL string      `json:"bodyURL"`
}

func logger(remoteIP, verb, line string) {
	log.Printf("%s %s %s\n", remoteIP, verb, line)
}

func DBSave(id string, in []byte, token string) error {
	dest := fmt.Sprintf("/usr/local/lib/maildb/%s.eml", id)
	f, err := os.Create(dest)
	if err != nil {
		return errors.Wrap(err, "could not create db file")
	}
	defer f.Close()

	f.WriteString(fmt.Sprintf("%s: %s%s", MW_BODY_SECRET_TOKEN, token, CRLF))
	f.Write(in)
	return nil
}

func Run(addr string) error {
	smtpd.Debug = true
	srv := &smtpd.Server{
		Addr:     addr,
		Handler:  mailHandler,
		Appname:  "webhook",
		Hostname: config.CurrConfig.InstanceHostname,
		Timeout:  3 * time.Minute,
		LogRead:  logger,
		LogWrite: logger,
	}
	log.Infof("Listening on %s", addr)
	return srv.ListenAndServe()
}

func callWebHook(wp *WebhookPayload, url string, id string, domain string, mailDate string, secret string) error {
	jsonData, err := json.Marshal(wp)
	if err != nil {
		return errors.Wrap(err, "could not serialize request payload")
	}

	// TODO: compute HMAC sha256
	signature := ""

	req, err := retryablehttp.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mw-Domain", domain)
	req.Header.Set("Mw-Id", id)
	req.Header.Set("Mw-Signature", signature)
	req.Header.Set("Mw-Date", mailDate)

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "could not send request")
	}
	defer resp.Body.Close()

	log.Infof("webhook returned: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode != 200 {
		return errors.Errorf("webhook returned %d %s", resp.StatusCode, resp.Status)
	} else {
		return nil
	}
}

func generateToken() string {
	token := make([]byte, 32)
	seededRand.Read(token)
	return fmt.Sprintf("%x", token)
}

func mailHandler(origin net.Addr, from string, to []string, in []byte) error {
	msg, err := mail.ReadMessage(bytes.NewReader(in))
	if err != nil {
		log.Errorf("failed to parse message: %s", err)
		return internalError
	}

	id := msg.Header.Get("Mw-Int-Id")
	bodyToken := generateToken()

	if err := DBSave(id, in, bodyToken); err != nil {
		log.Errorf("failed to save email buffer: %s", err)
		return internalError
	}
	if err := os.Remove(fmt.Sprintf("/tmp/%s.eml", id)); err != nil {
		log.Errorf("could not delete temporary file: %s", err)
	}

	secret := msg.Header.Get(MW_WEBHOOK_SECRET_TOKEN)
	endpoint := msg.Header.Get(MW_WEBHOOK_URL)
	bodyUrl := fmt.Sprintf("https://%s/db/email/%s?token=%s",
		config.CurrConfig.InstanceHostname, id, bodyToken)
	domain := msg.Header.Get("Mw-Int-Domain")
	mailDate := msg.Header.Get("Mw-Date")

	for key := range msg.Header {
		if strings.HasPrefix(strings.ToLower(key), INT_HEADER_PREFIX) {
			delete(msg.Header, key)
		}
	}

	data := WebhookPayload{
		Headers: msg.Header,
		BodyURL: bodyUrl,
	}

	if err := callWebHook(&data, endpoint, id, domain, mailDate, secret); err != nil {
		if err := updateMailStatus(config.CurrConfig.ServerJWT, domain, id, MAIL_STATUS_DELIVERY_ERROR); err != nil {
			log.Errorf("could not update email status in maildb: %s", err)
		}
		log.Warnf("could not call webhook: %s", err)
		return internalError
	}
	if err := updateMailStatus(config.CurrConfig.ServerJWT, domain, id, MAIL_STATUS_DELIVERED); err != nil {
		log.Errorf("could not update email status in maildb: %s", err)
	}
	return nil
}

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("failed to init config: %s", err)
	}

	httpClient = retryablehttp.NewClient()
	httpClient.RetryMax = 5
	httpClient.HTTPClient = &http.Client{
		Timeout: 60 * time.Second,
	}

	addr := fmt.Sprintf("127.0.0.1:%d", config.CurrConfig.PortWebhook)
	if err := Run(addr); err != nil {
		log.Fatal(err)
	}
}
