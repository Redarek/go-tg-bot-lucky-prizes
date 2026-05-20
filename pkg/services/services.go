package services

import (
	"context"
	"errors"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/models"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/repositories"
)

var ErrNoAttempts = errors.New("no_attempts")
var ErrNoAvailablePacks = errors.New("no_available_packs")

type Service struct {
	Repo *repositories.Repository
}

func NewService(repo *repositories.Repository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) ClaimStickerPack(ctx context.Context, userID int64, isAdmin bool) (models.StickerPack, error) {
	// Админ может дергать бесконечно
	if !isAdmin {
		pack, err := s.Repo.ClaimAvailableStickerPack(ctx, userID)
		if errors.Is(err, repositories.ErrNoAttempts) {
			return models.StickerPack{}, ErrNoAttempts
		}
		if errors.Is(err, repositories.ErrNoAvailablePacks) {
			return models.StickerPack{}, ErrNoAvailablePacks
		}
		return pack, err
	}
	return s.Repo.GetRandomStickerPack(ctx)
}
