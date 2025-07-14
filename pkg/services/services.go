package services

import (
	"context"
	"errors"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/models"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/repositories"
)

type Service struct {
	Repo *repositories.Repository
}

func NewService(repo *repositories.Repository) *Service {
	return &Service{Repo: repo}
}

func (s *Service) ClaimStickerPack(ctx context.Context, userID, adminID int64) (models.StickerPack, error) {
	if userID != adminID {
		if s.Repo.HasUserClaimed(ctx, userID) {
			return models.StickerPack{}, errors.New("Вы уже получили стикерпак")
		}
	}

	pack, err := s.Repo.GetRandomStickerPack(ctx)
	if err != nil {
		return models.StickerPack{}, err
	}

	err = s.Repo.MarkUserClaimed(ctx, userID, pack.ID)
	return pack, err
}
