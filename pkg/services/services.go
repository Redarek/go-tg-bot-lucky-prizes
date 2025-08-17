package services

import (
	"context"
	"errors"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/models"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/repositories"
)

var ErrAlreadyClaimed = errors.New("already_claimed")

type Service struct {
	Repo *repositories.Repository
}

func NewService(repo *repositories.Repository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) ClaimStickerPack(ctx context.Context, userID, adminID int64) (models.StickerPack, error) {
	// Админ может дергать бесконечно
	if userID != adminID {
		ok, err := s.Repo.TryClaim(ctx, userID)
		if err != nil {
			return models.StickerPack{}, err
		}
		if !ok {
			return models.StickerPack{}, ErrAlreadyClaimed
		}
	}
	return s.Repo.GetRandomStickerPack(ctx)
}
