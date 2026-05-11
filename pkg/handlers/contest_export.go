package handlers

import (
	"fmt"
	"time"

	"github.com/Redarek/go-tg-bot-lucky-prizes/pkg/models"
	"github.com/xuri/excelize/v2"
)

func buildContestParticipantsXLSX(contest models.Contest, rows []models.ContestParticipantExportRow) ([]byte, error) {
	f := excelize.NewFile()
	const sheet = "participants"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"contest_id", "user_id", "username", "first_name", "last_name", "profile_link", "joined_at"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(sheet, cell, header); err != nil {
			return nil, err
		}
	}

	for i, row := range rows {
		r := i + 2
		if err := f.SetCellInt(sheet, fmt.Sprintf("A%d", r), int64(contest.ID)); err != nil {
			return nil, err
		}
		if err := f.SetCellInt(sheet, fmt.Sprintf("B%d", r), row.UserID); err != nil {
			return nil, err
		}
		if err := f.SetCellStr(sheet, fmt.Sprintf("C%d", r), row.Username); err != nil {
			return nil, err
		}
		if err := f.SetCellStr(sheet, fmt.Sprintf("D%d", r), row.FirstName); err != nil {
			return nil, err
		}
		if err := f.SetCellStr(sheet, fmt.Sprintf("E%d", r), row.LastName); err != nil {
			return nil, err
		}
		profileLink := fmt.Sprintf("tg://user?id=%d", row.UserID)
		if row.Username != "" {
			profileLink = "https://t.me/" + row.Username
		}
		if err := f.SetCellStr(sheet, fmt.Sprintf("F%d", r), profileLink); err != nil {
			return nil, err
		}
		if err := f.SetCellStr(sheet, fmt.Sprintf("G%d", r), row.JoinedAt.UTC().Format(time.RFC3339)); err != nil {
			return nil, err
		}
	}

	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return nil, err
	}

	if err := f.SetColWidth(sheet, "A", "A", 12); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "B", "B", 18); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "C", "E", 20); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "F", "F", 34); err != nil {
		return nil, err
	}
	if err := f.SetColWidth(sheet, "G", "G", 30); err != nil {
		return nil, err
	}

	data, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
