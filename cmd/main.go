package main

import (
	"context"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/services"
	"golang.org/x/time/rate"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/config"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/db"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/handlers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg := config.LoadConfig()
	if cfg.TelegramToken == "" {
		log.Fatal("TELEGRAM_APITOKEN not found in config")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("Telegram init error: %v", err)
	}
	log.Printf("Authorized as @%s", bot.Self.UserName)

	pub := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Начать работу"},
		tgbotapi.BotCommand{Command: "draw", Description: "Получить стикерпак"},
	)
	publicScope := tgbotapi.NewBotCommandScopeDefault()
	pub.Scope = &publicScope
	_, _ = bot.Request(pub)

	admin := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Начать работу"},
		tgbotapi.BotCommand{Command: "packs", Description: "Список стикерпаков"},
		tgbotapi.BotCommand{Command: "addpack", Description: "Добавить стикерпак"},
	)
	adminScope := tgbotapi.NewBotCommandScopeChat(cfg.AdminID)
	admin.Scope = &adminScope
	_, _ = bot.Request(admin)

	pool := db.Connect(cfg)
	defer pool.Close()

	// Глобальный лимит Telegram. Ставим «безопасные» ~28 rps.
	lim := rate.NewLimiter(rate.Limit(28), 28)
	sender := services.NewSender(bot, lim)

	h := handlers.NewHandler(bot, sender, pool, cfg)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = []string{"message", "callback_query"} // меньше шума
	updates := bot.GetUpdatesChan(u)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Пул воркеров + очередь (бэкпрешер)
	const workers = 64
	jobs := make(chan tgbotapi.Update, 4096)
	for i := 0; i < workers; i++ {
		go func() {
			for upd := range jobs {
				// защита от паник внутри обработчика
				func() {
					defer func() { _ = recover() }()
					h.HandleUpdate(upd)
				}()
			}
		}()
	}

	log.Println("Bot started")

	for {
		select {
		case <-ctx.Done():
			close(jobs)
			return
		case upd, ok := <-updates:
			if !ok {
				log.Println("updates channel closed")
				close(jobs)
				return
			}
			select {
			case jobs <- upd:
			default:
				// Очередь переполнена — дропнем событие (или можно считать метрику)
				log.Println("updates backlog overflow, dropping update")
			}
		}
	}
}
