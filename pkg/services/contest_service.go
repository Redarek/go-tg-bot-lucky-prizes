package services

import (
	"context"
	"errors"

	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/models"
	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/repositories"
)

var ErrContestNotFound = errors.New("contest_not_found")
var ErrNoActiveContest = errors.New("no_active_contest")
var ErrNoParticipants = errors.New("no_participants")
var ErrActiveContestExists = errors.New("active_contest_exists")
var ErrAlreadyParticipant = errors.New("already_participant")
var ErrContestChannelsNotConfigured = errors.New("contest_channels_not_configured")
var ErrInvalidRewardPack = errors.New("invalid_reward_pack")

func (s *Service) CreateContest(
	ctx context.Context,
	title string,
	rewardPackID int,
	channel1ID int64,
	channel1Link string,
	channel2ID int64,
	channel2Link string,
) (models.Contest, error) {
	if channel1ID == 0 || channel2ID == 0 || channel1Link == "" || channel2Link == "" {
		return models.Contest{}, ErrContestChannelsNotConfigured
	}

	channels := []models.ContestChannel{
		{
			ChannelID:   channel1ID,
			ChannelLink: channel1Link,
			Position:    1,
		},
		{
			ChannelID:   channel2ID,
			ChannelLink: channel2Link,
			Position:    2,
		},
	}

	if _, err := s.Repo.GetStickerPackByID(ctx, rewardPackID); err != nil {
		if errors.Is(err, repositories.ErrNoPacks) {
			return models.Contest{}, ErrInvalidRewardPack
		}
		return models.Contest{}, err
	}

	contest, err := s.Repo.CreateContest(ctx, title, rewardPackID, channels)
	if errors.Is(err, repositories.ErrActiveContestExists) {
		return models.Contest{}, ErrActiveContestExists
	}
	return contest, err
}

func (s *Service) ActivateContest(ctx context.Context, contestID int) error {
	if _, err := s.Repo.GetContestByID(ctx, contestID); err != nil {
		if errors.Is(err, repositories.ErrContestNotFound) {
			return ErrContestNotFound
		}
		return err
	}

	if err := s.Repo.SetContestStatus(ctx, contestID, "active"); err != nil {
		if errors.Is(err, repositories.ErrActiveContestExists) {
			return ErrActiveContestExists
		}
		return err
	}
	return nil
}

func (s *Service) CloseActiveContest(ctx context.Context) (models.Contest, error) {
	contest, err := s.Repo.GetActiveContest(ctx)
	if err != nil {
		if errors.Is(err, repositories.ErrNoActiveContest) {
			return models.Contest{}, ErrNoActiveContest
		}
		return models.Contest{}, err
	}

	if err = s.Repo.SetContestStatus(ctx, contest.ID, "closed"); err != nil {
		return models.Contest{}, err
	}
	return contest, nil
}

func (s *Service) GetActiveContestWithChannels(ctx context.Context) (models.Contest, []models.ContestChannel, error) {
	contest, err := s.Repo.GetActiveContest(ctx)
	if err != nil {
		if errors.Is(err, repositories.ErrNoActiveContest) {
			return models.Contest{}, nil, ErrNoActiveContest
		}
		return models.Contest{}, nil, err
	}

	channels, err := s.Repo.GetContestChannels(ctx, contest.ID)
	if err != nil {
		return models.Contest{}, nil, err
	}
	return contest, channels, nil
}

func (s *Service) JoinActiveContest(
	ctx context.Context,
	userID int64,
	username string,
	firstName string,
	lastName string,
) (models.Contest, models.StickerPack, bool, error) {
	contest, err := s.Repo.GetActiveContest(ctx)
	if err != nil {
		if errors.Is(err, repositories.ErrNoActiveContest) {
			return models.Contest{}, models.StickerPack{}, false, ErrNoActiveContest
		}
		return models.Contest{}, models.StickerPack{}, false, err
	}

	rewardPack, err := s.Repo.GetStickerPackByID(ctx, contest.RewardPackID)
	if err != nil {
		if errors.Is(err, repositories.ErrNoPacks) {
			return models.Contest{}, models.StickerPack{}, false, ErrInvalidRewardPack
		}
		return models.Contest{}, models.StickerPack{}, false, err
	}

	added, err := s.Repo.AddParticipant(ctx, contest.ID, userID, username, firstName, lastName)
	if err != nil {
		return models.Contest{}, models.StickerPack{}, false, err
	}
	if !added {
		return contest, rewardPack, false, ErrAlreadyParticipant
	}
	return contest, rewardPack, true, nil
}

func (s *Service) PickWinnerInActiveContest(ctx context.Context) (models.Contest, models.ContestParticipantExportRow, error) {
	contest, err := s.Repo.GetActiveContest(ctx)
	if err != nil {
		if errors.Is(err, repositories.ErrNoActiveContest) {
			return models.Contest{}, models.ContestParticipantExportRow{}, ErrNoActiveContest
		}
		return models.Contest{}, models.ContestParticipantExportRow{}, err
	}

	winnerUserID, err := s.Repo.PickRandomWinner(ctx, contest.ID)
	if err != nil {
		if errors.Is(err, repositories.ErrNoParticipants) {
			return models.Contest{}, models.ContestParticipantExportRow{}, ErrNoParticipants
		}
		return models.Contest{}, models.ContestParticipantExportRow{}, err
	}

	if err = s.Repo.SetWinner(ctx, contest.ID, winnerUserID); err != nil {
		return models.Contest{}, models.ContestParticipantExportRow{}, err
	}

	winner, err := s.Repo.GetParticipantExportRow(ctx, contest.ID, winnerUserID)
	if err != nil {
		return models.Contest{}, models.ContestParticipantExportRow{}, err
	}
	return contest, winner, nil
}

func (s *Service) GetActiveContestParticipants(ctx context.Context) (models.Contest, []models.ContestParticipantExportRow, error) {
	contest, err := s.Repo.GetActiveContest(ctx)
	if err != nil {
		if errors.Is(err, repositories.ErrNoActiveContest) {
			return models.Contest{}, nil, ErrNoActiveContest
		}
		return models.Contest{}, nil, err
	}

	rows, err := s.Repo.ListParticipantsExportRows(ctx, contest.ID)
	if err != nil {
		return models.Contest{}, nil, err
	}
	return contest, rows, nil
}
