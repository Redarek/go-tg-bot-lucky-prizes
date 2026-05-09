package models

import "time"

type StickerPack struct {
	ID      int
	Name    string
	URL     string
	Deleted bool
}

type UserClaim struct {
	UserID int64
}

type AdminState struct {
	UserID int64
	State  string
	Data   string
}

type Contest struct {
	ID           int
	Title        string
	Status       string
	RewardPackID int
	WinnerUserID *int64
	CreatedAt    time.Time
	FinishedAt   *time.Time
}

type ContestChannel struct {
	ContestID   int
	ChannelID   int64
	ChannelLink string
	Position    int16
}

type ContestParticipant struct {
	ContestID int
	UserID    int64
	JoinedAt  time.Time
}

type ContestParticipantExportRow struct {
	UserID   int64
	JoinedAt time.Time
}
