package voice

import "go.mongodb.org/mongo-driver/v2/bson"

type Voice struct {
	ID        bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name      string        `json:"name,omitempty" bson:"name,omitempty"`
	VoiceType string        `json:"voiceType,omitempty" bson:"voice_type,omitempty"`
}
