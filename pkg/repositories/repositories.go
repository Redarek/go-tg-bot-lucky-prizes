package repositories

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoPacks = errors.New("no_packs")
var ErrNoAvailablePacks = errors.New("no_available_packs")
var ErrNoAttempts = errors.New("no_attempts")
var ErrNoActiveContest = errors.New("no_active_contest")
var ErrNoParticipants = errors.New("no_participants")
var ErrActiveContestExists = errors.New("active_contest_exists")
var ErrContestNotFound = errors.New("contest_not_found")

func init() { rand.Seed(time.Now().UnixNano()) }

type Repository struct {
	DB *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{DB: db} }

func (r *Repository) CreateStickerPack(ctx context.Context, name, url string) error {
	_, err := r.DB.Exec(ctx, `INSERT INTO sticker_packs (name, url) VALUES ($1, $2)`, name, url)
	return err
}

func (r *Repository) UpdateStickerPack(ctx context.Context, id int, name, url string) error {
	_, err := r.DB.Exec(ctx, `UPDATE sticker_packs SET name=$1, url=$2 WHERE id=$3`, name, url, id)
	return err
}

func (r *Repository) DeleteStickerPack(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM sticker_packs WHERE id=$1`, id)
	return err
}

func (r *Repository) GetStickerPacks(ctx context.Context) ([]models.StickerPack, error) {
	rows, err := r.DB.Query(ctx, `SELECT id, name, url FROM sticker_packs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.StickerPack
	for rows.Next() {
		var p models.StickerPack
		if err := rows.Scan(&p.ID, &p.Name, &p.URL); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *Repository) GetRandomStickerPack(ctx context.Context) (models.StickerPack, error) {
	var p models.StickerPack
	err := r.DB.QueryRow(ctx,
		`SELECT id, name, url FROM sticker_packs ORDER BY RANDOM() LIMIT 1`).
		Scan(&p.ID, &p.Name, &p.URL)

	if errors.Is(err, pgx.ErrNoRows) {
		return models.StickerPack{}, errors.New("список стикерпаков пуст")
	}
	return p, err
}

func (r *Repository) ClaimAvailableStickerPack(ctx context.Context, userID int64) (models.StickerPack, error) {
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.StickerPack{}, err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `
		INSERT INTO user_attempts (user_id, attempts) VALUES ($1, 1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return models.StickerPack{}, err
	}

	var attempts int
	if err = tx.QueryRow(ctx, `SELECT attempts FROM user_attempts WHERE user_id=$1 FOR UPDATE`, userID).Scan(&attempts); err != nil {
		return models.StickerPack{}, err
	}
	if attempts <= 0 {
		return models.StickerPack{}, ErrNoAttempts
	}

	var p models.StickerPack
	err = tx.QueryRow(ctx, `
		SELECT sp.id, sp.name, sp.url
		FROM sticker_packs sp
		WHERE NOT EXISTS (
			SELECT 1
			FROM user_pack_claims upc
			WHERE upc.user_id = $1 AND upc.pack_id = sp.id
		)
		ORDER BY RANDOM()
		LIMIT 1
	`, userID).Scan(&p.ID, &p.Name, &p.URL)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.StickerPack{}, ErrNoAvailablePacks
	}
	if err != nil {
		return models.StickerPack{}, err
	}

	if _, err = tx.Exec(ctx, `
		UPDATE user_attempts
		SET attempts = attempts - 1
		WHERE user_id = $1
	`, userID); err != nil {
		return models.StickerPack{}, err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO user_pack_claims (user_id, pack_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, pack_id) DO NOTHING
	`, userID, p.ID); err != nil {
		return models.StickerPack{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return models.StickerPack{}, err
	}
	return p, nil
}

func (r *Repository) AddAttemptToAllUsers(ctx context.Context) (int64, error) {
	ct, err := r.DB.Exec(ctx, `
		INSERT INTO user_attempts (user_id, attempts)
		SELECT user_id, 1
		FROM bot_users
		ON CONFLICT (user_id)
		DO UPDATE SET attempts = user_attempts.attempts + 1
	`)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

func (r *Repository) SetAdminState(ctx context.Context, st models.AdminState) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO admin_states (user_id, state, data)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id) DO UPDATE SET state=$2, data=$3`,
		st.UserID, st.State, st.Data)
	return err
}

func (r *Repository) GetAdminState(ctx context.Context, userID int64) (models.AdminState, error) {
	var st models.AdminState
	err := r.DB.QueryRow(ctx, `SELECT user_id, state, data FROM admin_states WHERE user_id=$1`,
		userID).Scan(&st.UserID, &st.State, &st.Data)

	if errors.Is(err, pgx.ErrNoRows) {
		return models.AdminState{}, nil
	}
	return st, err
}

func (r *Repository) ClearAdminState(ctx context.Context, userID int64) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM admin_states WHERE user_id=$1`, userID)
	return err
}

func (r *Repository) UpsertBotUser(
	ctx context.Context,
	userID int64,
	username string,
	firstName string,
	lastName string,
) error {
	_, err := r.DB.Exec(ctx,
		`INSERT INTO bot_users (user_id, username, first_name, last_name, last_seen_at)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), now())
		ON CONFLICT (user_id) DO UPDATE
		SET
			username = COALESCE(NULLIF(EXCLUDED.username, ''), bot_users.username),
			first_name = COALESCE(NULLIF(EXCLUDED.first_name, ''), bot_users.first_name),
			last_name = COALESCE(NULLIF(EXCLUDED.last_name, ''), bot_users.last_name),
			last_seen_at = now()`,
		userID, username, firstName, lastName)
	return err
}

func (r *Repository) UpsertMessageTarget(ctx context.Context, userID int64) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO bot_message_targets (user_id, can_message, last_confirmed_at, updated_at)
		VALUES ($1, TRUE, now(), now())
		ON CONFLICT (user_id) DO UPDATE
		SET
			can_message = TRUE,
			last_confirmed_at = now(),
			last_error_code = NULL,
			last_error_text = NULL,
			blocked_at = NULL,
			updated_at = now()
	`, userID)
	return err
}

func (r *Repository) CreateContest(
	ctx context.Context,
	title string,
	rewardPackID int,
	channels []models.ContestChannel,
) (models.Contest, error) {
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.Contest{}, err
	}
	defer tx.Rollback(ctx)

	var c models.Contest
	err = tx.QueryRow(ctx, `
		INSERT INTO contests (title, status, reward_pack_id)
		VALUES ($1, 'draft', $2)
		RETURNING id, title, status, reward_pack_id, winner_user_id, created_at, finished_at
	`, title, rewardPackID).Scan(
		&c.ID,
		&c.Title,
		&c.Status,
		&c.RewardPackID,
		&c.WinnerUserID,
		&c.CreatedAt,
		&c.FinishedAt,
	)
	if err != nil {
		return models.Contest{}, err
	}

	for _, channel := range channels {
		_, err = tx.Exec(ctx, `
			INSERT INTO contest_channels (contest_id, channel_id, channel_link, position)
			VALUES ($1, $2, $3, $4)
		`, c.ID, channel.ChannelID, channel.ChannelLink, channel.Position)
		if err != nil {
			return models.Contest{}, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return models.Contest{}, err
	}
	return c, nil
}

func (r *Repository) GetContestByID(ctx context.Context, contestID int) (models.Contest, error) {
	var c models.Contest
	err := r.DB.QueryRow(ctx, `
		SELECT id, title, status, reward_pack_id, winner_user_id, created_at, finished_at
		FROM contests
		WHERE id = $1
	`, contestID).Scan(
		&c.ID,
		&c.Title,
		&c.Status,
		&c.RewardPackID,
		&c.WinnerUserID,
		&c.CreatedAt,
		&c.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Contest{}, ErrContestNotFound
	}
	return c, err
}

func (r *Repository) GetActiveContest(ctx context.Context) (models.Contest, error) {
	var c models.Contest
	err := r.DB.QueryRow(ctx, `
		SELECT id, title, status, reward_pack_id, winner_user_id, created_at, finished_at
		FROM contests
		WHERE status='active'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&c.ID,
		&c.Title,
		&c.Status,
		&c.RewardPackID,
		&c.WinnerUserID,
		&c.CreatedAt,
		&c.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Contest{}, ErrNoActiveContest
	}
	return c, err
}

func (r *Repository) SetContestStatus(ctx context.Context, contestID int, status string) error {
	switch status {
	case "active":
		tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		var activeCount int
		if err = tx.QueryRow(ctx, `SELECT COUNT(1) FROM contests WHERE status='active' AND id <> $1`, contestID).Scan(&activeCount); err != nil {
			return err
		}
		if activeCount > 0 {
			return ErrActiveContestExists
		}

		_, err = tx.Exec(ctx, `UPDATE contests SET status='active', finished_at=NULL WHERE id=$1`, contestID)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)

	case "draft":
		_, err := r.DB.Exec(ctx, `UPDATE contests SET status='draft', finished_at=NULL WHERE id=$1`, contestID)
		return err
	case "closed":
		_, err := r.DB.Exec(ctx, `UPDATE contests SET status='closed', finished_at=now() WHERE id=$1`, contestID)
		return err
	default:
		return errors.New("invalid contest status")
	}
}

func (r *Repository) GetContestChannels(ctx context.Context, contestID int) ([]models.ContestChannel, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT contest_id, channel_id, channel_link, position
		FROM contest_channels
		WHERE contest_id = $1
		ORDER BY position ASC
	`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.ContestChannel
	for rows.Next() {
		var c models.ContestChannel
		if err := rows.Scan(&c.ContestID, &c.ChannelID, &c.ChannelLink, &c.Position); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

func (r *Repository) GetStickerPackByID(ctx context.Context, id int) (models.StickerPack, error) {
	var p models.StickerPack
	err := r.DB.QueryRow(ctx, `
		SELECT id, name, url
		FROM sticker_packs
		WHERE id=$1
	`, id).Scan(&p.ID, &p.Name, &p.URL)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.StickerPack{}, ErrNoPacks
	}
	return p, err
}

func (r *Repository) AddParticipant(
	ctx context.Context,
	contestID int,
	userID int64,
	username string,
	firstName string,
	lastName string,
) (bool, error) {
	ct, err := r.DB.Exec(ctx, `
		INSERT INTO contest_participants (contest_id, user_id, username, first_name, last_name)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''))
		ON CONFLICT (contest_id, user_id) DO NOTHING
	`, contestID, userID, username, firstName, lastName)
	if err != nil {
		return false, err
	}
	if ct.RowsAffected() == 1 {
		return true, nil
	}

	_, err = r.DB.Exec(ctx, `
		UPDATE contest_participants
		SET
			username = COALESCE(NULLIF($3, ''), username),
			first_name = COALESCE(NULLIF($4, ''), first_name),
			last_name = COALESCE(NULLIF($5, ''), last_name)
		WHERE contest_id = $1 AND user_id = $2
	`, contestID, userID, username, firstName, lastName)
	if err != nil {
		return false, err
	}
	return false, nil
}

func (r *Repository) ListParticipants(ctx context.Context, contestID int) ([]models.ContestParticipant, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT contest_id, user_id, COALESCE(username, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), joined_at
		FROM contest_participants
		WHERE contest_id = $1
		ORDER BY joined_at ASC
	`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ContestParticipant
	for rows.Next() {
		var p models.ContestParticipant
		if err := rows.Scan(&p.ContestID, &p.UserID, &p.Username, &p.FirstName, &p.LastName, &p.JoinedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *Repository) ListParticipantsExportRows(ctx context.Context, contestID int) ([]models.ContestParticipantExportRow, error) {
	rows, err := r.DB.Query(ctx, `
		SELECT user_id, COALESCE(username, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), joined_at
		FROM contest_participants
		WHERE contest_id = $1
		ORDER BY joined_at ASC
	`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ContestParticipantExportRow
	for rows.Next() {
		var p models.ContestParticipantExportRow
		if err := rows.Scan(&p.UserID, &p.Username, &p.FirstName, &p.LastName, &p.JoinedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *Repository) GetParticipantExportRow(ctx context.Context, contestID int, userID int64) (models.ContestParticipantExportRow, error) {
	var p models.ContestParticipantExportRow
	err := r.DB.QueryRow(ctx, `
		SELECT user_id, COALESCE(username, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), joined_at
		FROM contest_participants
		WHERE contest_id = $1 AND user_id = $2
		LIMIT 1
	`, contestID, userID).Scan(&p.UserID, &p.Username, &p.FirstName, &p.LastName, &p.JoinedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ContestParticipantExportRow{}, ErrNoParticipants
	}
	return p, err
}

func (r *Repository) PickRandomWinner(ctx context.Context, contestID int) (int64, error) {
	var userID int64
	err := r.DB.QueryRow(ctx, `
		SELECT user_id
		FROM contest_participants
		WHERE contest_id = $1
		ORDER BY RANDOM()
		LIMIT 1
	`, contestID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoParticipants
	}
	return userID, err
}

func (r *Repository) SetWinner(ctx context.Context, contestID int, winnerUserID int64) error {
	_, err := r.DB.Exec(ctx, `
		UPDATE contests
		SET winner_user_id = $2
		WHERE id = $1
	`, contestID, winnerUserID)
	return err
}
