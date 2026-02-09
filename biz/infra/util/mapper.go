package util

import "go.mongodb.org/mongo-driver/v2/bson"

func ObjectIDsFromHex(ids ...string) ([]bson.ObjectID, error) {
	var objectIDs []bson.ObjectID
	for _, id := range ids {
		oid, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}
		objectIDs = append(objectIDs, oid)
	}
	return objectIDs, nil
}
