package models

type StickerPack struct {
	ID   int
	Name string
	URL  string
}

type UserClaim struct {
	UserID        int64
	StickerPackID int
}

type AdminState struct {
	UserID int64
	State  string
	Data   string
}
