package handlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/config"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/models"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/repositories"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/services"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	bot            *tgbotapi.BotAPI
	service        *services.Service
	adminID        int64
	shopURL        string
	subChannelID   string
	subChannelLink string
}

func NewHandler(bot *tgbotapi.BotAPI, db *pgxpool.Pool, cfg *config.Config) *Handler {
	repo := repositories.NewRepository(db)
	return &Handler{
		bot:            bot,
		service:        services.NewService(repo),
		adminID:        cfg.AdminID,
		shopURL:        cfg.ShopURL,
		subChannelID:   cfg.SubChannelID,
		subChannelLink: cfg.SubChannelLink,
	}
}

func (h *Handler) HandleUpdate(upd tgbotapi.Update) {
	ctx := context.Background()

	if upd.Message != nil {

		if upd.Message.IsCommand() && upd.Message.From.ID == h.adminID {
			h.handleAdminCommand(ctx, upd.Message)
			return
		}

		if upd.Message.IsCommand() &&
			upd.Message.From.ID != h.adminID &&
			upd.Message.Command() == "draw" {

			h.processDraw(ctx, upd.Message.Chat.ID, upd.Message.From.ID)
			return
		}

		if upd.Message.IsCommand() &&
			upd.Message.From.ID != h.adminID &&
			upd.Message.Command() == "start" {
			h.sendStartMessage(upd.Message.Chat.ID)
			return
		}

		h.handleAdminDialog(ctx, upd.Message)
		return
	}

	if upd.CallbackQuery != nil {
		h.handleCallback(ctx, upd.CallbackQuery)
	}
}

func (h *Handler) sendStartMessage(chatID int64) {
	mk := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Разыграть стикерпак", "draw"),
			tgbotapi.NewInlineKeyboardButtonURL("Получить все", h.shopURL),
		))

	msg := tgbotapi.NewMessage(chatID, "Добро пожаловать! Выбери действие:")
	msg.ReplyMarkup = mk
	h.bot.Send(msg)
}

func (h *Handler) handleCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	switch {
	case q.Data == "start":
		h.sendStartMessage(q.Message.Chat.ID)

	case q.Data == "draw":
		h.processDraw(ctx, q.Message.Chat.ID, q.Message.From.ID)

	case strings.HasPrefix(q.Data, "pack_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "pack_"))
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✏️ Редактировать", fmt.Sprintf("edit_%d", id)),
				tgbotapi.NewInlineKeyboardButtonData("🗑️ Удалить", fmt.Sprintf("del_%d", id)),
			))
		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Что сделать со стикерпаком?")
		msg.ReplyMarkup = mk
		h.bot.Send(msg)

	case strings.HasPrefix(q.Data, "del_"):
		id := strings.TrimPrefix(q.Data, "del_")
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", "delok_"+id),
			))

		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Точно удалить?")
		msg.ReplyMarkup = mk
		h.bot.Send(msg)

	case strings.HasPrefix(q.Data, "delok_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "delok_"))
		if err := h.service.Repo.DeleteStickerPack(ctx, id); err != nil {
			h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID,
				"Ошибка удаления: "+err.Error()))
		} else {
			h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID, "✅ Удалено"))
		}

	case strings.HasPrefix(q.Data, "edit_"):
		id := strings.TrimPrefix(q.Data, "edit_")
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: q.From.ID, State: "edit_wait_name", Data: id,
		})
		h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID,
			"Отправьте новое НАЗВАНИЕ:"))
	}
}

func (h *Handler) handleAdminCommand(ctx context.Context, m *tgbotapi.Message) {
	switch m.Command() {
	case "start":
		h.sendStartMessage(m.Chat.ID)

	case "packs":
		h.showPacksList(ctx, m.Chat.ID)

	case "addpack":
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_name",
		})
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID,
			"Отправьте НАЗВАНИЕ нового стикерпака:"))

	case "draw":
		h.processDraw(ctx, m.Chat.ID, m.From.ID)
	}
}

func (h *Handler) showPacksList(ctx context.Context, chatID int64) {
	packs, _ := h.service.Repo.GetStickerPacks(ctx)
	if len(packs) == 0 {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Стикерпаков нет"))
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range packs {
		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("[%d] %s", p.ID, p.Name),
			fmt.Sprintf("pack_%d", p.ID))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	mk := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, "Выберите стикерпак:")
	msg.ReplyMarkup = mk
	h.bot.Send(msg)
}

func (h *Handler) handleAdminDialog(ctx context.Context, m *tgbotapi.Message) {
	st, _ := h.service.Repo.GetAdminState(ctx, m.From.ID)

	switch st.State {

	case "add_wait_name":
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_url", Data: m.Text,
		})
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте ссылку:"))

	case "add_wait_url":
		if err := h.service.Repo.CreateStickerPack(ctx, st.Data, m.Text); err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			return
		}
		h.service.Repo.ClearAdminState(ctx, m.From.ID)
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ Стикерпак добавлен"))

	case "edit_wait_name":
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: m.From.ID,
			State:  "edit_wait_url",
			Data:   st.Data + "|" + m.Text,
		})
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте новую ссылку:"))

	case "edit_wait_url":
		parts := strings.SplitN(st.Data, "|", 2)
		id, _ := strconv.Atoi(parts[0])
		newName := parts[1]
		newURL := m.Text
		if err := h.service.Repo.UpdateStickerPack(ctx, id, newName, newURL); err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			return
		}
		h.service.Repo.ClearAdminState(ctx, m.From.ID)
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ Обновлено"))
	}
}

func (h *Handler) subscribed(userID int64) bool {
	if h.subChannelID == "" {
		return true
	}

	cfg := tgbotapi.ChatConfigWithUser{UserID: userID}

	if id, err := strconv.ParseInt(h.subChannelID, 10, 64); err == nil {
		cfg.ChatID = id
	} else {
		cfg.SuperGroupUsername = strings.TrimPrefix(h.subChannelID, "@")
	}

	member, err := h.bot.GetChatMember(tgbotapi.GetChatMemberConfig{ChatConfigWithUser: cfg})
	if err != nil {
		log.Println("GetChatMember:", err)
		return false
	}

	switch member.Status {
	case "creator", "administrator", "member":
		return true
	default:
		return false
	}
}

func (h *Handler) processDraw(ctx context.Context, chatID, userID int64) {
	if !h.subscribed(userID) {
		h.bot.Send(tgbotapi.NewMessage(chatID,
			"Сначала подпишитесь на канал "+h.subChannelLink))
		return
	}

	p, err := h.service.ClaimStickerPack(ctx, userID, h.adminID)
	if err != nil {
		if strings.Contains(err.Error(), "Список стикерпаков пуст") {
			h.bot.Send(tgbotapi.NewMessage(chatID,
				"⚠️ Стикерпаков пока нет. Попробуйте позже."))
			return
		}
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Получить все стикерпаки", h.shopURL),
			))

		msg := tgbotapi.NewMessage(chatID, err.Error())
		msg.ReplyMarkup = mk
		h.bot.Send(msg)
		return
	}

	dice := tgbotapi.NewDice(chatID)
	dice.Emoji = "🎲" // есть ещё 🎲 ⚽ 🏀 🎳 🎯🎰
	msg, _ := h.bot.Send(dice)

	time.Sleep(2 * time.Second)

	text := "Ваш стикерпак: " + p.URL

	res := tgbotapi.NewMessage(chatID, text)
	res.ReplyToMessageID = msg.MessageID

	h.bot.Send(res)
}
