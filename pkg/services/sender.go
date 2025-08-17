package services

import (
	"context"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/time/rate"
)

type Sender struct {
	bot *tgbotapi.BotAPI
	lim *rate.Limiter
}

func NewSender(bot *tgbotapi.BotAPI, lim *rate.Limiter) *Sender {
	return &Sender{bot: bot, lim: lim}
}

// Глобальный лимит на любой исходящий вызов
func (s *Sender) Wait(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.lim.Wait(ctx)
}

// Отправка сообщений (Chattable)
func (s *Sender) Send(ctx context.Context, msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	if err := s.Wait(ctx); err != nil {
		var empty tgbotapi.Message
		return empty, err
	}
	return s.bot.Send(msg)
}
