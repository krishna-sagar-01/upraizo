package service

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"server/internal/broker"
	"server/internal/config"
	"server/internal/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	amqp "github.com/rabbitmq/amqp091-go"
)

// StartEmailConsumer spins up a background goroutine that consumes from
// the email queue and sends each message through AWS SES using templated
// emails.
//
// Call this once during application startup (after broker.Connect and
// EmailService.DeclareQueue have succeeded).
//
// On individual message failure the message is requeued so it can be
// retried. Malformed messages (unmarshal errors) are nack'd without
// requeue to avoid poison-pill loops.
func StartEmailConsumer(cfg *config.Config) error {
	// 1. Dedicated consumer channel (best-practice: separate from producer channel)
	ch, err := broker.Conn.Channel()
	if err != nil {
		utils.Error("Failed to open consumer channel", err, nil)
		return fmt.Errorf("consumer channel: %w", err)
	}

	// 2. QoS — process one message at a time for back-pressure control
	if err = ch.Qos(1, 0, false); err != nil {
		utils.Error("Failed to set QoS on consumer channel", err, nil)
		return fmt.Errorf("consumer QoS: %w", err)
	}

	// 3. Register consumer
	msgs, err := ch.Consume(
		EmailQueue, // queue
		"",         // consumer tag (auto-generated)
		false,      // auto-ack disabled — we ack/nack manually
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,
	)
	if err != nil {
		utils.Error("Failed to register email consumer", err, nil)
		return fmt.Errorf("consumer register: %w", err)
	}

	// 4. Initialise AWS SES client
	awsSess, err := session.NewSession(&aws.Config{
		Region: aws.String(cfg.AWS.AWSRegion),
		Credentials: credentials.NewStaticCredentials(
			cfg.AWS.AWSAccessKey,
			cfg.AWS.AWSSecretKey,
			"",
		),
	})
	if err != nil {
		utils.Error("Failed to create AWS session for SES", err, nil)
		return fmt.Errorf("aws session: %w", err)
	}

	sesClient := ses.New(awsSess)
	fromEmail := cfg.AWS.AWSFromEmail

	utils.Info("Email consumer started", map[string]any{"queue": EmailQueue})

	// 5. Process messages in a background goroutine
	go func() {
		for msg := range msgs {
			// ── Retry Tracking ──────────────────────────────
			retryCount := 0
			if val, ok := msg.Headers["x-retry-count"]; ok {
				if count, ok := val.(int32); ok {
					retryCount = int(count)
				} else if count, ok := val.(int); ok {
					retryCount = count
				}
			}

			// ── Unmarshal ────────────────────────────────────
			var email EmailMessage
			if err := json.Unmarshal(msg.Body, &email); err != nil {
				utils.Error("Failed to unmarshal email message", err, map[string]any{
					"body": string(msg.Body),
				})
				_ = msg.Nack(false, false) // poison pill — don't requeue
				continue
			}

			// ── Build SES template data (JSON string) ────────
			templateDataJSON, err := json.Marshal(email.TemplateData)
			if err != nil {
				utils.Error("Failed to marshal template data", err, nil)
				_ = msg.Nack(false, false)
				continue
			}

			// ── Send via SES ─────────────────────────────────
			input := &ses.SendTemplatedEmailInput{
				Source: aws.String(fromEmail),
				Destination: &ses.Destination{
					ToAddresses: []*string{aws.String(email.To)},
				},
				Template:     aws.String(email.TemplateName),
				TemplateData: aws.String(string(templateDataJSON)),
			}

			if _, err = sesClient.SendTemplatedEmail(input); err != nil {
				utils.Error("SES SendTemplatedEmail failed", err, map[string]any{
					"to":          email.To,
					"template":    email.TemplateName,
					"retry_count": retryCount,
				})

				// ── Permanent Error Check ──
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					case ses.ErrCodeMessageRejected:
						utils.Warn("Permanent SES failure: discarding message to avoid infinite loop", map[string]any{
							"to":    email.To,
							"error": aerr.Message(),
						})
						_ = msg.Nack(false, false)
						continue
					}
				}

				// ── Retry Logic (Max 3) with Exponential Backoff ──
				if retryCount < 3 {
					newRetryCount := retryCount + 1
					// Backoff: 10s, 20s, 40s
					delay := time.Duration(math.Pow(2, float64(newRetryCount-1))) * 10 * time.Second

					utils.Info("Retrying email delivery with backoff", map[string]any{
						"to":      email.To,
						"attempt": newRetryCount,
						"delay":   delay.String(),
						"max":     3,
					})

					// Update headers
					headers := msg.Headers
					if headers == nil {
						headers = make(map[string]any)
					}
					headers["x-retry-count"] = int32(newRetryCount)

					// Execute retry in a background goroutine to avoid blocking the consumer QoS
					go func(h amqp.Table, b []byte, ct string, d time.Duration) {
						time.Sleep(d)
						err := ch.Publish(
							"",         // exchange
							EmailQueue, // routing key
							false,      // mandatory
							false,      // immediate
							amqp.Publishing{
								Headers:     h,
								ContentType: ct,
								Body:        b,
							},
						)
						if err != nil {
							utils.Error("Failed to re-publish for retry after backoff", err, nil)
						}
					}(headers, msg.Body, msg.ContentType, delay)

					_ = msg.Ack(false) // remove original message after scheduling re-publish
				} else {
					utils.Error("Max email retries reached. Discarding message.", nil, map[string]any{
						"to": email.To,
					})
					_ = msg.Nack(false, false)
				}
				continue
			}

			utils.Info("Email sent via SES", map[string]any{
				"to":       email.To,
				"template": email.TemplateName,
			})
			_ = msg.Ack(false)
		}

		// Channel closed — log for observability
		utils.Warn("Email consumer channel closed", nil)
	}()

	return nil
}
