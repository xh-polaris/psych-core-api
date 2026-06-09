package util

import (
	"fmt"
	"time"
)

var CST = time.FixedZone("CST", 8*3600)

func DayToUTCRange(date string) (time.Time, time.Time, error) {
	dayStart, err := time.ParseInLocation("2006-01-02", date, CST)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse date %s: %w", date, err)
	}
	dayEnd := dayStart.Add(24 * time.Hour)
	return dayStart.UTC(), dayEnd.UTC(), nil
}

func ParseDayRange(startDate, endDate string) (time.Time, time.Time, error) {
	start, _, err := DayToUTCRange(startDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endDayEnd, _, err := DayToUTCRange(endDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, endDayEnd, nil
}

func FormatDateUTC8(t time.Time) string {
	return t.In(CST).Format("2006-01-02")
}

func DateToTimestampUTC8(date string) (int64, error) {
	t, err := time.ParseInLocation("2006-01-02", date, CST)
	if err != nil {
		return 0, fmt.Errorf("parse date %s: %w", date, err)
	}
	return t.Unix(), nil
}
