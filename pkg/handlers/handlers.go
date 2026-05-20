package handlers

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html"
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

//go:embed assets/start.jpeg
var StartJPG []byte

type Handler struct {
	bot                 *tgbotapi.BotAPI
	sender              *services.Sender
	service             *services.Service
	adminIDs            map[int64]struct{}
	shopURL             string
	subChannelID        int64
	subChannelLink      string
	contestChannel1ID   int64
	contestChannel1Link string
	contestChannel2ID   int64
	contestChannel2Link string
}

func NewHandler(bot *tgbotapi.BotAPI, sender *services.Sender, db *pgxpool.Pool, cfg *config.Config) *Handler {
	repo := repositories.NewRepository(db)
	adminIDs := make(map[int64]struct{}, len(cfg.AdminIDs))
	for _, id := range cfg.AdminIDs {
		adminIDs[id] = struct{}{}
	}
	return &Handler{
		bot:                 bot,
		sender:              sender,
		service:             services.NewService(repo),
		adminIDs:            adminIDs,
		shopURL:             cfg.ShopURL,
		subChannelID:        cfg.SubChannelID,
		subChannelLink:      cfg.SubChannelLink,
		contestChannel1ID:   cfg.ContestChannel1ID,
		contestChannel1Link: cfg.ContestChannel1Link,
		contestChannel2ID:   cfg.ContestChannel2ID,
		contestChannel2Link: cfg.ContestChannel2Link,
	}
}

func (h *Handler) isAdmin(userID int64) bool {
	_, ok := h.adminIDs[userID]
	return ok
}

func (h *Handler) HandleUpdate(upd tgbotapi.Update) {
	// базовый контекст на обработку одного апдейта
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch {
	case upd.Message != nil:
		m := upd.Message
		h.trackUserInteraction(ctx, m.From, m.Chat.IsPrivate())

		// Сначала админские команды
		if m.IsCommand() && m.From != nil && h.isAdmin(m.From.ID) {
			h.handleAdminCommand(ctx, m)
			return
		}

		// Пользовательские команды
		if m.IsCommand() && m.From != nil && !h.isAdmin(m.From.ID) {
			switch m.Command() {
			case "draw":
				h.processDraw(ctx, m.Chat.ID, m.From.ID)
				return
			case "contest":
				h.processContestJoin(ctx, m.Chat.ID, m.From)
				return
			case "start":
				h.sendStartMessage(ctx, m.Chat.ID)
				return
			}
		}

		// Диалог админа — только для админа (чтобы не бить БД по каждому юзеру)
		if m.From != nil && h.isAdmin(m.From.ID) {
			h.handleAdminDialog(ctx, m)
		}

	case upd.CallbackQuery != nil:
		isPrivate := false
		if upd.CallbackQuery.Message != nil {
			isPrivate = upd.CallbackQuery.Message.Chat.IsPrivate()
		}
		h.trackUserInteraction(ctx, upd.CallbackQuery.From, isPrivate)
		h.handleCallback(ctx, upd.CallbackQuery)
	}
}

func (h *Handler) sendStartMessage(ctx context.Context, chatID int64) {
	mk := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Получить стикерпак", "draw"),
		),
	)

	contestCtx, contestCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer contestCancel()
	hasActiveGiveaway := false
	if _, _, err := h.service.GetActiveContestWithChannels(contestCtx); err == nil {
		hasActiveGiveaway = true
	}

	caption := "🎯<b><u>Готов испытать свою удачу?</u></b>\n" +
		"Запускай Колесо Фортуны и забирай один из <i>фирменных ультра-брутальных</i> стикерпаков <b>TWILIGHT HAMMER!</b>\n" +
		"☸️<i>Крути колесо, боец! Забери свой трофей!</i>"

	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: "start.jpg", Bytes: StartJPG})
	photo.Caption = caption
	photo.ReplyMarkup = mk
	photo.ParseMode = tgbotapi.ModeHTML
	if _, err := h.sender.Send(ctx, photo); err != nil {
		log.Println("sendStartMessage:", err)
	}

	if !hasActiveGiveaway {
		return
	}

	giveawayMk := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("УЧАСТВОВАТЬ", "contest_join"),
		),
	)
	giveawayText := fmt.Sprintf(
		"🔥<b>АБСОЛЮТНЫЙ РОЗЫГРЫШ</b>🔥\n\n" +
			"⚔️ Врывайся в бой за трофеи <b>TWILIGHT HAMMER</b>.\n" +
			"☠️ Правила просты: подпишись на 2 канала, нажми кнопку и закрепи своё место в списке бойцов.\n" +
			"🎁 За подтверждённое участие получаешь награду сразу.\n\n" +
			"<b>Розыгрыш уже открыт — жми и залетай.</b>",
	)

	giveawayMsg := tgbotapi.NewMessage(chatID, giveawayText)
	giveawayMsg.ParseMode = tgbotapi.ModeHTML
	giveawayMsg.ReplyMarkup = giveawayMk
	if _, err := h.sender.Send(ctx, giveawayMsg); err != nil {
		log.Println("sendStartMessage giveaway:", err)
	}
}

func (h *Handler) handleCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	// всегда отвечаем на callback, чтобы убрать "часики"
	if q.ID != "" {
		_, _ = h.bot.Request(tgbotapi.NewCallback(q.ID, ""))
	}

	// Бывают инлайн-коллбэки без Message
	if q.Message == nil {
		return
	}

	switch {
	case q.Data == "start":
		h.sendStartMessage(ctx, q.Message.Chat.ID)

	case q.Data == "draw":
		h.processDraw(ctx, q.Message.Chat.ID, q.From.ID)

	case q.Data == "contest_join", q.Data == "contest_recheck":
		h.processContestJoin(ctx, q.Message.Chat.ID, q.From)

	case q.Data == "contest_close_confirm":
		if !h.isAdmin(q.From.ID) {
			return
		}
		dbctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		defer cancel()
		_, err := h.service.CloseActiveContest(dbctx)
		if err != nil {
			if errors.Is(err, services.ErrNoActiveContest) {
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "Активного розыгрыша нет."))
				return
			}
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "Ошибка закрытия розыгрыша: "+err.Error()))
			return
		}
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "✅ Розыгрыш закрыт вручную. Список участников сохранён в базе."))

	case q.Data == "contest_close_cancel":
		if !h.isAdmin(q.From.ID) {
			return
		}
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "Ок, закрытие розыгрыша отменено."))

	case strings.HasPrefix(q.Data, "pack_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "pack_"))
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✏️ Редактировать", fmt.Sprintf("edit_%d", id)),
				tgbotapi.NewInlineKeyboardButtonData("🗑️ Удалить", fmt.Sprintf("del_%d", id)),
			))
		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Что сделать со стикерпаком?")
		msg.ReplyMarkup = mk
		if _, err := h.sender.Send(ctx, msg); err != nil {
			log.Println(err)
		}

	case strings.HasPrefix(q.Data, "del_"):
		id := strings.TrimPrefix(q.Data, "del_")
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Да, удалить", "delok_"+id),
			))
		msg := tgbotapi.NewMessage(q.Message.Chat.ID, "Точно удалить?")
		msg.ReplyMarkup = mk
		if _, err := h.sender.Send(ctx, msg); err != nil {
			log.Println(err)
		}

	case strings.HasPrefix(q.Data, "delok_"):
		id, _ := strconv.Atoi(strings.TrimPrefix(q.Data, "delok_"))
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if err := h.service.Repo.DeleteStickerPack(dbctx, id); err != nil {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "Ошибка удаления: "+err.Error()))
		} else {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "✅ Удалено"))
		}

	case strings.HasPrefix(q.Data, "edit_"):
		id := strings.TrimPrefix(q.Data, "edit_")
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: q.From.ID, State: "edit_wait_name", Data: id,
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(q.Message.Chat.ID, "Отправьте новое название:"))
	}
}

func (h *Handler) handleAdminCommand(ctx context.Context, m *tgbotapi.Message) {
	switch m.Command() {
	case "start":
		h.sendStartMessage(ctx, m.Chat.ID)
	case "packs":
		h.showPacksList(ctx, m.Chat.ID)
	case "addpack":
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_name",
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Отправьте название нового стикерпака:"))
	case "addattempt":
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		updated, err := h.service.Repo.AddAttemptToAllUsers(dbctx)
		if err != nil {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка добавления попыток: "+err.Error()))
			return
		}
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ Добавлена 1 попытка всем пользователям (обновлено записей: %d)", updated)))
	case "draw":
		h.processDraw(ctx, m.Chat.ID, m.From.ID)
	case "contestadd":
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: m.From.ID, State: "contest_add_wait_title",
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Отправьте название розыгрыша:"))
	case "conteststart":
		contestID, err := strconv.Atoi(strings.TrimSpace(m.CommandArguments()))
		if err != nil || contestID <= 0 {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Использование: /conteststart <contest_id>"))
			return
		}
		dbctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		defer cancel()
		if err = h.service.ActivateContest(dbctx, contestID); err != nil {
			switch {
			case errors.Is(err, services.ErrActiveContestExists):
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Нельзя активировать: уже есть активный розыгрыш. Закройте его командой /contestclose."))
			case errors.Is(err, services.ErrContestNotFound):
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Розыгрыш не найден."))
			default:
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка активации розыгрыша: "+err.Error()))
			}
			return
		}
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "✅ Розыгрыш активирован."))
	case "contestclose":
		dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if _, _, err := h.service.GetActiveContestWithChannels(dbctx); err != nil {
			if errors.Is(err, services.ErrNoActiveContest) {
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Активного розыгрыша нет."))
				return
			}
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Не удалось проверить активный розыгрыш: "+err.Error()))
			return
		}

		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить закрытие", "contest_close_confirm"),
				tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "contest_close_cancel"),
			),
		)
		msg := tgbotapi.NewMessage(m.Chat.ID, "Закрыть активный розыгрыш? Участники не удаляются, но новый вход в розыгрыш будет остановлен.")
		msg.ReplyMarkup = mk
		_, _ = h.sender.Send(ctx, msg)
	case "contestpickwinner":
		dbctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		_, winner, err := h.service.PickWinnerInActiveContest(dbctx)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrNoActiveContest):
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Активного розыгрыша нет."))
			case errors.Is(err, services.ErrNoParticipants):
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "У активного розыгрыша пока нет участников."))
			default:
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка выбора победителя: "+err.Error()))
			}
			return
		}

		chatCtx, chatCancel := context.WithTimeout(ctx, 2*time.Second)
		winner = h.enrichParticipantFromChat(chatCtx, winner)
		chatCancel()

		winnerMsg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(
			"🏆 <b>Победитель розыгрыша выбран</b>\n<b>Профиль победителя:</b>\n%s\n\nРозыгрыш остаётся активным до ручного закрытия.",
			formatParticipantContactHTML(winner),
		))
		winnerMsg.ParseMode = tgbotapi.ModeHTML
		_, _ = h.sender.Send(ctx, winnerMsg)
	case "contestparticipants":
		dbctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		contest, rows, err := h.service.GetActiveContestParticipants(dbctx)
		if err != nil {
			if errors.Is(err, services.ErrNoActiveContest) {
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Активного розыгрыша нет."))
				return
			}
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка выгрузки участников: "+err.Error()))
			return
		}
		if len(rows) == 0 {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "У активного розыгрыша пока нет участников."))
			return
		}

		fileData, err := buildContestParticipantsXLSX(contest, rows)
		if err != nil {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Не удалось сформировать XLSX: "+err.Error()))
			return
		}

		doc := tgbotapi.NewDocument(m.Chat.ID, tgbotapi.FileBytes{
			Name:  fmt.Sprintf("contest_%d_participants.xlsx", contest.ID),
			Bytes: fileData,
		})
		doc.Caption = "Участники активного розыгрыша"
		if _, err = h.sender.Send(ctx, doc); err != nil {
			log.Println("contestparticipants:", err)
		}
	}
}

func (h *Handler) showPacksList(ctx context.Context, chatID int64) {
	dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	packs, err := h.service.Repo.GetStickerPacks(dbctx)
	if err != nil {
		log.Println("GetStickerPacks:", err)
		return
	}
	if len(packs) == 0 {
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Стикерпаков не добавлено"))
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, p := range packs {
		btn := tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("[%d] %s", p.ID, p.Name), fmt.Sprintf("pack_%d", p.ID))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	mk := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, "Выберите стикерпак:")
	msg.ReplyMarkup = mk
	_, _ = h.sender.Send(ctx, msg)
}

func (h *Handler) handleAdminDialog(ctx context.Context, m *tgbotapi.Message) {
	dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	st, _ := h.service.Repo.GetAdminState(dbctx, m.From.ID)

	switch st.State {

	case "add_wait_name":
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: m.From.ID, State: "add_wait_url", Data: m.Text,
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте ссылку:"))

	case "add_wait_url":
		if err := h.service.Repo.CreateStickerPack(dbctx, st.Data, m.Text); err != nil {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			return
		}
		_ = h.service.Repo.ClearAdminState(dbctx, m.From.ID)
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "✅ Стикерпак добавлен"))

	case "edit_wait_name":
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: m.From.ID, State: "edit_wait_url", Data: st.Data + "|" + m.Text,
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте новую ссылку:"))

	case "edit_wait_url":
		parts := strings.SplitN(st.Data, "|", 2)
		id, _ := strconv.Atoi(parts[0])
		newName := parts[1]
		newURL := m.Text
		if err := h.service.Repo.UpdateStickerPack(dbctx, id, newName, newURL); err != nil {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка: "+err.Error()))
			return
		}
		_ = h.service.Repo.ClearAdminState(dbctx, m.From.ID)
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "✅ Обновлено"))

	case "contest_add_wait_title":
		_ = h.service.Repo.SetAdminState(dbctx, models.AdminState{
			UserID: m.From.ID, State: "contest_add_wait_reward_pack", Data: m.Text,
		})
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Теперь отправьте reward pack id (число):"))

	case "contest_add_wait_reward_pack":
		rewardPackID, err := strconv.Atoi(strings.TrimSpace(m.Text))
		if err != nil || rewardPackID <= 0 {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "reward pack id должен быть положительным числом."))
			return
		}

		contest, err := h.service.CreateContest(
			dbctx,
			st.Data,
			rewardPackID,
			h.contestChannel1ID,
			normalizeTelegramChannelLink(h.contestChannel1Link),
			h.contestChannel2ID,
			normalizeTelegramChannelLink(h.contestChannel2Link),
		)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrContestChannelsNotConfigured):
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Настройки каналов розыгрыша не заданы (CONTEST_CHANNEL_1/2_*)."))
			case errors.Is(err, services.ErrInvalidRewardPack):
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Такого reward pack id нет в sticker_packs."))
			case errors.Is(err, services.ErrActiveContestExists):
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Нельзя создать новый розыгрыш, пока есть активный. Закройте активный командой /contestclose."))
			default:
				_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(m.Chat.ID, "Ошибка создания розыгрыша: "+err.Error()))
			}
			return
		}

		_ = h.service.Repo.ClearAdminState(dbctx, m.From.ID)
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(
			m.Chat.ID,
			fmt.Sprintf("✅ Розыгрыш создан в статусе draft.\nID розыгрыша: %d\nЗапуск: /conteststart %d", contest.ID, contest.ID),
		))
	}
}

func (h *Handler) subscribed(ctx context.Context, userID int64) bool {
	if h.subChannelID == 0 {
		return true
	}
	// Учитываем общий лимит Telegram
	if err := h.sender.Wait(ctx); err != nil {
		log.Println("rate wait:", err)
		return false
	}

	cfg := tgbotapi.ChatConfigWithUser{ChatID: h.subChannelID, UserID: userID}
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

func (h *Handler) subscribedToChannel(ctx context.Context, userID, channelID int64) bool {
	if channelID == 0 {
		return false
	}
	if err := h.sender.Wait(ctx); err != nil {
		log.Println("rate wait:", err)
		return false
	}

	cfg := tgbotapi.ChatConfigWithUser{ChatID: channelID, UserID: userID}
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

func (h *Handler) getMissingContestChannels(ctx context.Context, userID int64, channels []models.ContestChannel) []models.ContestChannel {
	var missing []models.ContestChannel
	for _, ch := range channels {
		if !h.subscribedToChannel(ctx, userID, ch.ChannelID) {
			missing = append(missing, ch)
		}
	}
	return missing
}

func (h *Handler) processContestJoin(ctx context.Context, chatID int64, user *tgbotapi.User) {
	if user == nil {
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Не удалось определить пользователя. Попробуйте снова."))
		return
	}
	userID := user.ID

	dbctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	_, channels, err := h.service.GetActiveContestWithChannels(dbctx)
	if err != nil {
		if errors.Is(err, services.ErrNoActiveContest) {
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Сейчас нет активного розыгрыша."))
			return
		}
		log.Println("GetActiveContestWithChannels:", err)
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Ошибка получения данных розыгрыша. Попробуйте позже."))
		return
	}
	if len(channels) < 2 {
		_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Розыгрыш временно недоступен: не настроены обязательные каналы."))
		return
	}

	subCtx, subCancel := context.WithTimeout(ctx, 4*time.Second)
	defer subCancel()
	missing := h.getMissingContestChannels(subCtx, userID, channels)
	if len(missing) > 0 {
		rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(missing)+1)
		for _, ch := range missing {
			link := normalizeTelegramChannelLink(ch.ChannelLink)
			if link == "" {
				continue
			}
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Подписаться: "+ch.ChannelLink, link),
			))
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Проверить снова", "contest_recheck"),
		))
		msg := tgbotapi.NewMessage(chatID, "Для участия в розыгрыше нужно подписаться на оба канала.")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		_, _ = h.sender.Send(ctx, msg)
		return
	}

	joinCtx, joinCancel := context.WithTimeout(ctx, 700*time.Millisecond)
	defer joinCancel()
	_, rewardPack, _, err := h.service.JoinActiveContest(joinCtx, userID, user.UserName, user.FirstName, user.LastName)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAlreadyParticipant):
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Ты уже участвуешь в этом розыгрыше ✅"))
		case errors.Is(err, services.ErrNoActiveContest):
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Сейчас нет активного розыгрыша."))
		case errors.Is(err, services.ErrInvalidRewardPack):
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Награда розыгрыша пока недоступна, попробуйте позже."))
		default:
			log.Println("JoinActiveContest:", err)
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Не удалось подтвердить участие. Попробуйте позже."))
		}
		return
	}

	text := fmt.Sprintf(
		"⚔️<b>БОЕЦ ПРИНЯТ В РОЗЫГРЫШ</b>\n"+
			"🔥 Подписка подтверждена. Твой жетон участия выбит в сталь.\n"+
			"🎁 <b>Трофей за вход в бой:</b>\n%s\n\n"+
			"☠️ Держи строй и не теряй хватку — дальше только жёстче.",
		rewardPack.URL,
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	_, _ = h.sender.Send(ctx, msg)
}

func normalizeTelegramChannelLink(link string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}
	if strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "tg://") {
		return link
	}
	if strings.HasPrefix(link, "@") && len(link) > 1 {
		return "https://t.me/" + strings.TrimPrefix(link, "@")
	}
	return "https://t.me/" + link
}

func (h *Handler) trackUserInteraction(ctx context.Context, user *tgbotapi.User, canMessage bool) {
	if user == nil {
		return
	}

	dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if err := h.service.Repo.UpsertBotUser(dbctx, user.ID, user.UserName, user.FirstName, user.LastName); err != nil {
		log.Println("UpsertBotUser:", err)
		return
	}

	if !canMessage {
		return
	}

	if err := h.service.Repo.UpsertMessageTarget(dbctx, user.ID); err != nil {
		log.Println("UpsertMessageTarget:", err)
	}
}

func formatParticipantContactHTML(p models.ContestParticipantExportRow) string {
	displayName := strings.TrimSpace(strings.TrimSpace(p.FirstName + " " + p.LastName))
	if displayName == "" {
		displayName = "Без имени"
	}
	displayName = html.EscapeString(displayName)

	profileLink := fmt.Sprintf("tg://user?id=%d", p.UserID)
	if p.Username != "" {
		profileLink = fmt.Sprintf("https://t.me/%s", p.Username)
	}

	usernamePart := "username: —"
	if p.Username != "" {
		usernamePart = fmt.Sprintf("username: @%s", html.EscapeString(p.Username))
	}

	return fmt.Sprintf(
		"• id: <code>%d</code>\n• имя: %s\n• %s\n• <a href=\"%s\">Открыть профиль</a>",
		p.UserID,
		displayName,
		usernamePart,
		profileLink,
	)
}

func (h *Handler) enrichParticipantFromChat(ctx context.Context, p models.ContestParticipantExportRow) models.ContestParticipantExportRow {
	if err := h.sender.Wait(ctx); err != nil {
		return p
	}

	chat, err := h.bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: p.UserID},
	})
	if err != nil {
		return p
	}

	if chat.UserName != "" {
		p.Username = chat.UserName
	}
	if chat.FirstName != "" {
		p.FirstName = chat.FirstName
	}
	if chat.LastName != "" {
		p.LastName = chat.LastName
	}
	return p
}

func (h *Handler) processDraw(ctx context.Context, chatID, userID int64) {
	// Проверка подписки
	subCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	if !h.subscribed(subCtx, userID) {
		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Проверить подписку", "draw"),
			))
		msg := tgbotapi.NewMessage(chatID, "Подпишись на канал "+h.subChannelLink+", чтобы получить стикерпак")
		msg.ReplyMarkup = mk
		_, _ = h.sender.Send(ctx, msg)
		return
	}

	// Клейм + выбор пакета
	dbctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	p, err := h.service.ClaimStickerPack(dbctx, userID, h.isAdmin(userID))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrNoAttempts):
			mk := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("Заказать броню", h.shopURL),
				))
			msg := tgbotapi.NewMessage(chatID,
				"⚡️<u>Попытка была одна — и Фортуна уже выбрала стикерпак под твой стиль!</u>\n"+
					"🔄Хочешь другой? Тогда заказывай нашу броню TWILIGHT HAMMER и получай в бонус фирменный стикерпак, который идёт в комплекте с экипировкой.\n\n"+
					"<b>Заказать можешь тут:</b>\n"+
					"🟣<b><a href=\"https://www.wildberries.ru/brands/311439225-twilight-hammer\">WILDBERRIES</a></b>\n"+
					"🔵<b><a href=\"https://vk.com/t.hammer.clan\">VKONTAKTE</a></b>")
			msg.ParseMode = tgbotapi.ModeHTML
			msg.ReplyMarkup = mk
			_, _ = h.sender.Send(ctx, msg)
			return
		case errors.Is(err, services.ErrNoAvailablePacks):
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "⚠️ Ты уже получил все доступные стикерпаки. Жди пополнение и новую попытку."))
			return
		case errors.Is(err, repositories.ErrNoPacks):
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "⚠️ Стикерпаков пока нет. Попробуйте позже."))
			return
		default:
			log.Println("ClaimStickerPack:", err)
			_, _ = h.sender.Send(ctx, tgbotapi.NewMessage(chatID, "Произошла ошибка. Попробуйте позже."))
			return
		}
	}

	// Отправляем "кубик" сразу…
	dice := tgbotapi.NewDice(chatID)
	dice.Emoji = "🎲"
	_, _ = h.sender.Send(ctx, dice)

	// …а дальше — без блокировки текущего воркера
	go func(chatID int64, url, shop string) {
		goCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		time.Sleep(2 * time.Second)

		text := "😎<b>НИШТЯК!</b> Ты залутал крутой стикерпак!\n" +
			"⚔️Теперь у тебя в руках оружие для чатов — <i>бей словами, жги эмоциями, взрывай переписки!</i>\n\n" + url
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = tgbotapi.ModeHTML
		_, _ = h.sender.Send(goCtx, msg)

		time.Sleep(1 * time.Second)

		mk := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("Заказать броню", shop),
			))
		after := "⚡️<u>Попытка была одна — и Фортуна уже выбрала стикерпак под твой стиль!</u>\n" +
			"🔄Хочешь другой? Тогда заказывай нашу броню TWILIGHT HAMMER и получай в бонус фирменный стикерпак, который идёт в комплекте с экипировкой.\n\n" +
			"<b>Заказать можешь тут:</b>\n" +
			"🟣<b><a href=\"https://www.wildberries.ru/brands/311439225-twilight-hammer\">WILDBERRIES</a></b>\n" +
			"🔵<b><a href=\"https://vk.com/t.hammer.clan\">VKONTAKTE</a></b>"

		am := tgbotapi.NewMessage(chatID, after)
		am.ParseMode = tgbotapi.ModeHTML
		am.ReplyMarkup = mk
		_, _ = h.sender.Send(goCtx, am)
	}(chatID, p.URL, h.shopURL)
}
