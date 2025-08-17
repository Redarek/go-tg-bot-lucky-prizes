package handlers

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/config"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/models"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/repositories"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/services"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"strconv"
	"strings"
	"time"
)

//go:embed assets/start.jpeg
var StartJPG []byte

type Handler struct {
	bot            *tgbotapi.BotAPI
	service        *services.Service
	adminID        int64
	shopURL        string
	subChannelID   int64
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
	err := h.service.Repo.UpsertBotUser(context.Background(), chatID)
	if err != nil {
		log.Println(err)
		return
	}

	mk := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Получить стикерпак", "draw"),
		))

	caption := "🎯<b><u>Готов испытать свою удачу?</u></b>\n" +
		"Запускай Колесо Фортуны и забирай один из <i>фирменных ультра-брутальных</i> стикерпаков <b>TWILIGHT HAMMER!</b>\n" +
		"☸️<i>Крути колесо, боец! Забери свой трофей!</i>"

	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
		Name:  "start.jpg",
		Bytes: StartJPG,
	})
	photo.Caption = caption
	photo.ReplyMarkup = mk
	photo.ParseMode = tgbotapi.ModeHTML

	if _, err := h.bot.Send(photo); err != nil {
		_ = err
	}
}

func (h *Handler) handleCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	switch {
	case q.Data == "start":
		h.sendStartMessage(q.Message.Chat.ID)

	case q.Data == "draw":
		h.processDraw(ctx, q.Message.Chat.ID, q.From.ID)

	case strings.HasPrefix(q.Data, "pack_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "pack_"))
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✏️ Редактировать", fmt.Sprintf("edit_%d", id)),
				tgbotapi.NewInlineKeyboardButtonData("🗑️ Удалить", fmt.Sprintf("del_%d", id)),
			))
		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Что сделать со стикерпаком?")
		msg.ReplyMarkup = mk
		_, err := h.bot.Send(msg)
		if err != nil {
			log.Println(err)
			return
		}

	case strings.HasPrefix(q.Data, "del_"):
		id := strings.TrimPrefix(q.Data, "del_")
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", "delok_"+id),
			))

		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Точно удалить?")
		msg.ReplyMarkup = mk
		_, err := h.bot.Send(msg)
		if err != nil {
			log.Println(err)
			return
		}

	case strings.HasPrefix(q.Data, "delok_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "delok_"))
		if err := h.service.Repo.DeleteStickerPack(ctx, id); err != nil {
			_, err = h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID,
				"Ошибка удаления: "+err.Error()))
			if err != nil {
				log.Println(err)
				return
			}
		} else {
			_, err = h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID, "✅ Удалено"))
			if err != nil {
				log.Println(err)
				return
			}
		}

	case strings.HasPrefix(q.Data, "edit_"):
		id := strings.TrimPrefix(q.Data, "edit_")
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: q.From.ID, State: "edit_wait_name", Data: id,
		})
		_, err := h.bot.Send(tgbotapi.NewMessage(q.Message.Chat.ID,
			"Отправьте новое название:"))
		if err != nil {
			log.Println(err)
			return
		}
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
		_, err := h.bot.Send(tgbotapi.NewMessage(m.Chat.ID,
			"Отправьте название нового стикерпака:"))
		if err != nil {
			log.Println(err)
			return
		}

	case "draw":
		h.processDraw(ctx, m.Chat.ID, m.From.ID)
	}
}

func (h *Handler) showPacksList(ctx context.Context, chatID int64) {
	packs, _ := h.service.Repo.GetStickerPacks(ctx)
	if len(packs) == 0 {
		_, err := h.bot.Send(tgbotapi.NewMessage(chatID, "Стикерпаков не добавлено"))
		if err != nil {
			log.Println(err)
			return
		}
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
	_, err := h.bot.Send(msg)
	if err != nil {
		log.Println(err)
		return
	}
}

func (h *Handler) handleAdminDialog(ctx context.Context, m *tgbotapi.Message) {
	st, _ := h.service.Repo.GetAdminState(ctx, m.From.ID)

	switch st.State {

	case "add_wait_name":
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_url", Data: m.Text,
		})
		_, err := h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте ссылку:"))
		if err != nil {
			log.Println(err)
			return
		}

	case "add_wait_url":
		if err := h.service.Repo.CreateStickerPack(ctx, st.Data, m.Text); err != nil {
			_, err = h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			if err != nil {
				log.Println(err)
				return
			}
			return
		}
		err := h.service.Repo.ClearAdminState(ctx, m.From.ID)
		if err != nil {
			log.Println(err)
			return
		}
		_, err = h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ Стикерпак добавлен"))
		if err != nil {
			log.Println(err)
			return
		}

	case "edit_wait_name":
		_ = h.service.Repo.SetAdminState(ctx, models.AdminState{
			UserID: m.From.ID,
			State:  "edit_wait_url",
			Data:   st.Data + "|" + m.Text,
		})
		_, err := h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте новую ссылку:"))
		if err != nil {
			log.Println(err)
			return
		}

	case "edit_wait_url":
		parts := strings.SplitN(st.Data, "|", 2)
		id, _ := strconv.Atoi(parts[0])
		newName := parts[1]
		newURL := m.Text
		if err := h.service.Repo.UpdateStickerPack(ctx, id, newName, newURL); err != nil {
			_, err = h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			if err != nil {
				log.Println(err)
				return
			}
			return
		}
		err := h.service.Repo.ClearAdminState(ctx, m.From.ID)
		if err != nil {
			log.Println(err)
			return
		}
		_, err = h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ Обновлено"))
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func (h *Handler) subscribed(userID int64) bool {
	if h.subChannelID == 0 {
		return true
	}

	cfg := tgbotapi.ChatConfigWithUser{
		ChatID: h.subChannelID,
		UserID: userID,
	}

	member, err := h.bot.GetChatMember(tgbotapi.GetChatMemberConfig{ChatConfigWithUser: cfg})
	if err != nil {
		log.Println(err)
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
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Проверить подписку", "draw"),
			))
		msg := tgbotapi.NewMessage(chatID, "Подпишись на канал "+h.subChannelLink+", чтобы получить стикерпак")
		msg.ReplyMarkup = mk
		_, err := h.bot.Send(msg)
		if err != nil {
			log.Println(err)
			return
		}
		return
	}

	p, err := h.service.ClaimStickerPack(ctx, userID, h.adminID)
	if err != nil {
		if strings.Contains(err.Error(), "Список стикерпаков пуст") {
			_, err = h.bot.Send(tgbotapi.NewMessage(chatID,
				"⚠️ Стикерпаков пока нет. Попробуйте позже."))
			if err != nil {
				log.Println(err)
				return
			}
			return
		}
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Заказать броню", h.shopURL),
			))

		msg := tgbotapi.NewMessage(chatID, err.Error())
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = mk
		_, err = h.bot.Send(msg)
		if err != nil {
			log.Println(err)
			return
		}
		return
	}

	dice := tgbotapi.NewDice(chatID)
	dice.Emoji = "🎲" // есть ещё 🎲 ⚽ 🏀 🎳 🎯🎰
	_, err = h.bot.Send(dice)
	if err != nil {
		log.Println(err)
		return
	}

	time.Sleep(2 * time.Second)

	text := "😎<b>НИШТЯК!</b> Ты залутал крутой стикерпак!\n" +
		"⚔️Теперь у тебя в руках оружие для чатов — <i>бей словами, жги эмоциями, взрывай переписки!</i>\n\n" + p.URL

	res := tgbotapi.NewMessage(chatID, text)
	res.ParseMode = tgbotapi.ModeHTML
	_, err = h.bot.Send(res)
	if err != nil {
		log.Println(err)
		return
	}

	time.Sleep(1 * time.Second)
	mk := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Заказать броню", h.shopURL),
		))
	textAfterDraw := "⚡️<u>Попытка была одна — и Фортуна уже выбрала стикерпак под твой стиль!</u>\n" +
		"🔄Хочешь другой? Тогда заказывай нашу броню TWILIGHT HAMMER и получай в бонус фирменный стикерпак, который идёт в комплекте с экипировкой.\n\n" +
		"<b>Заказать можешь тут:</b>\n" +
		"🟣<b><a href=\"https://www.wildberries.ru/brands/311439225-twilight-hammer\">WILDBERRIES</a></b>\n" +
		"🔵<b><a href=\"https://vk.com/t.hammer.clan\">VKONTAKTE</a></b>"
	resAfterDraw := tgbotapi.NewMessage(chatID, textAfterDraw)
	resAfterDraw.ParseMode = tgbotapi.ModeHTML
	resAfterDraw.ReplyMarkup = mk
	_, err = h.bot.Send(resAfterDraw)
	if err != nil {
		log.Println(err)
		return
	}
}
