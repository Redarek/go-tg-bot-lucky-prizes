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

	headers := []string{"contest_id", "user_id", "joined_at"}
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
		if err := f.SetCellStr(sheet, fmt.Sprintf("C%d", r), row.JoinedAt.UTC().Format(time.RFC3339)); err != nil {
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
	if err := f.SetColWidth(sheet, "C", "C", 30); err != nil {
		return nil, err
	}

	data, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
