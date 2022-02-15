package time

import "time"

var Loc *time.Location

func init() {
	Loc, _ = time.LoadLocation("Asia/Shanghai")
}

func NowTimeWithLoc() time.Time {
	return time.Now().In(Loc)
}
