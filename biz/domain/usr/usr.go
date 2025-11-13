package usr

type Meta struct {
	UserId string `json:"user_id"`
	UnitId string `json:"unit_id;omitempty"`
	Code   string `json:"code;omitempty"`
}
