package usr

type Meta struct {
	UserId    string `json:"user_id"`
	UnitId    string `json:"unit_id;omitempty"`
	StudentId string `json:"student_id;omitempty"`
	Strong    bool   `json:"strong,omitempty"`
}
