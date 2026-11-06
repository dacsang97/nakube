package ui

import "time"

// Age is a short age, empty when when is zero. A clock in the future is 0s,
// not a negative age.
func Age(when, now time.Time) string {
	if when.IsZero() {
		return ""
	}
	d := now.Sub(when)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return itoa(int(d.Seconds())) + "s"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + "h"
	default:
		return itoa(int(d.Hours()/24)) + "d"
	}
}

// When is a short age and a clock time, empty when at is zero.
func When(at, now time.Time) string {
	age := Age(at, now)
	if age == "" {
		return ""
	}
	return age + " · " + at.Local().Format("Jan 2 15:04")
}
