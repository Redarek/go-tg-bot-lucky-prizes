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

func init() { rand.Seed(time.Now().UnixNano()) }

type Repository struct {
	DB *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{DB: db} }

func (r *Repository) CreateStickerPack(ctx context.Context, name, url string) error {
	_, err := r.DB.Exec(ctx,
		`INSERT INTO sticker_packs (name, url) VALUES ($1, $2)`, name, url)
	return err
}

func (r *Repository) UpdateStickerPack(ctx context.Context, id int, name, url string) error {
	_, err := r.DB.Exec(ctx,
		`UPDATE sticker_packs SET name=$1, url=$2 WHERE id=$3`, name, url, id)
	return err
}

func (r *Repository) DeleteStickerPack(ctx context.Context, id int) error {
	_, err := r.DB.Exec(ctx, `DELETE FROM sticker_packs WHERE id=$1`, id)
	return err
}

func (r *Repository) GetStickerPacks(ctx context.Context) ([]models.StickerPack, error) {
	rows, err := r.DB.Query(ctx,
		`SELECT id, name, url FROM sticker_packs ORDER BY id`)
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

func (r *Repository) HasUserClaimed(ctx context.Context, userID int64) bool {
	var exists bool
	_ = r.DB.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_claims WHERE user_id=$1)`, userID).
		Scan(&exists)
	return exists
}

func (r *Repository) MarkUserClaimed(ctx context.Context, userID int64, packID int) error {
	_, err := r.DB.Exec(ctx,
		`INSERT INTO user_claims (user_id, sticker_pack_id) VALUES ($1,$2)`,
		userID, packID)
	return err
}

func (r *Repository) SetAdminState(ctx context.Context, st models.AdminState) error {
	_, err := r.DB.Exec(ctx, `
		INSERT INTO admin_states (user_id, state, data)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id)
		DO UPDATE SET state=$2, data=$3`,
		st.UserID, st.State, st.Data)
	return err
}

func (r *Repository) GetAdminState(ctx context.Context, userID int64) (models.AdminState, error) {
	var st models.AdminState
	err := r.DB.QueryRow(ctx,
		`SELECT user_id, state, data FROM admin_states WHERE user_id=$1`,
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
