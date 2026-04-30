package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"server/internal/broker"
	"server/internal/config"
	"server/internal/utils"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const (
	// EmailQueue is the durable RabbitMQ queue that carries outbound email jobs.
	EmailQueue = "email_queue"

	// Template names — update these to match the actual AWS SES template names.
	VerifyTemplate = "VerifyEmail"
	ResetTemplate  = "ResetPassword"
	ResetSecretTemplate = "ResetSecretKey"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// EmailMessage is the JSON payload published to RabbitMQ.
type EmailMessage struct {
	To           string            `json:"to"`
	TemplateName string            `json:"template_name"`
	TemplateData map[string]string `json:"template_data"`
}

// EmailService produces email jobs onto the RabbitMQ queue.
// Actual sending is handled by the consumer goroutine (consumer.go).
type EmailService struct {
	cfg *config.Config
}

// NewEmailService returns a ready-to-use producer.
func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{cfg: cfg}
}

// ─── Queue Declaration ───────────────────────────────────────────────────────

// DeclareQueue declares the email queue (idempotent).
// Call this once during application startup before publishing or consuming.
func (s *EmailService) DeclareQueue() error {
	_, err := broker.Channel.QueueDeclare(
		EmailQueue,
		true,  // durable — survives broker restart
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		utils.Error("Failed to declare email queue", err, nil)
		return fmt.Errorf("email queue declare: %w", err)
	}
	utils.Info("Email queue declared", map[string]any{"queue": EmailQueue})
	return nil
}

// ─── Producers ───────────────────────────────────────────────────────────────

// SendVerificationEmail queues an email-verification job.
// Template data: name, token, link (full frontend URL with token).
func (s *EmailService) SendVerificationEmail(name, email, token string) error {
	link := fmt.Sprintf("%s?token=%s", s.cfg.Frontend.VerificationUrl, token)

	return s.publishToQueue(EmailMessage{
		To:           email,
		TemplateName: VerifyTemplate,
		TemplateData: map[string]string{
			"user_name":  name,
			"verification_url": link,
		},
	})
}

// SendPasswordResetEmail queues a password-reset email job.
// Template data: name, token, link (full frontend URL with token).
func (s *EmailService) SendPasswordResetEmail(name, email, token string) error {
	link := fmt.Sprintf("%s?token=%s", s.cfg.Frontend.ResetUrl, token)

	return s.publishToQueue(EmailMessage{
		To:           email,
		TemplateName: ResetTemplate,
		TemplateData: map[string]string{
			"user_name":  name,
			"reset_url": link,
		},
	})
}

// SendAdminResetPasswordEmail queues an admin password-reset email job.
// Uses a specific URL for admin password recovery.
func (s *EmailService) SendAdminResetPasswordEmail(name, email, token string) error {
	link := fmt.Sprintf("%s?token=%s", s.cfg.Frontend.AdminResetPasswordUrl, token)

	return s.publishToQueue(EmailMessage{
		To:           email,
		TemplateName: ResetTemplate, // Reusing ResetTemplate with different context
		TemplateData: map[string]string{
			"user_name":  name,
			"reset_url": link,
		},
	})
}

// SendSecretResetEmail queues a secret-key-reset email job.
// Uses a specific URL for secret key recovery.
func (s *EmailService) SendSecretResetEmail(name, email, token string) error {
	link := fmt.Sprintf("%s?token=%s", s.cfg.Frontend.SecretResetUrl, token)

	return s.publishToQueue(EmailMessage{
		To:           email,
		TemplateName: ResetTemplate, // Reusing ResetTemplate with different context
		TemplateData: map[string]string{
			"user_name":  name,
			"reset_url": link,
		},
	})
}

// ─── Internal ────────────────────────────────────────────────────────────────

// publishToQueue marshals the message and publishes it to the email queue
// with persistent delivery mode so it survives a broker restart.
func (s *EmailService) publishToQueue(msg EmailMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		utils.Error("Failed to marshal email message", err, nil)
		return utils.Internal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = broker.Channel.PublishWithContext(ctx,
		"",         // default exchange
		EmailQueue, // routing key = queue name
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		utils.Error("Failed to publish email to queue", err, map[string]any{
			"to":       msg.To,
			"template": msg.TemplateName,
		})
		return utils.Internal(err)
	}

	utils.Debug("Email queued successfully", map[string]any{
		"to":       msg.To,
		"template": msg.TemplateName,
	})
	return nil
}
