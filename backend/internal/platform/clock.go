package platform

import "time"

// Beijing is the sole wall-clock location for GoJira. All persisted timestamps
// must come from Now() — never time.Now().UTC().
var Beijing *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	Beijing = loc
}

// Now returns the current instant in Asia/Shanghai.
func Now() time.Time {
	return time.Now().In(Beijing)
}

// Location returns Asia/Shanghai.
func Location() *time.Location {
	return Beijing
}

// DateInBeijing constructs a timezone-aware date at midnight GMT+8.
func DateInBeijing(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, Beijing)
}

// ParseDate accepts YYYY-MM-DD in Asia/Shanghai.
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, Beijing)
}

// StartOfDay truncates t to midnight in Asia/Shanghai.
func StartOfDay(t time.Time) time.Time {
	t = t.In(Beijing)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, Beijing)
}
