package util

import "go.mongodb.org/mongo-driver/bson/primitive"

func ObjectIDsFromHex(ids ...string) ([]primitive.ObjectID, error) {
	var objectIDs []primitive.ObjectID
	for _, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}
		objectIDs = append(objectIDs, oid)
	}
	return objectIDs, nil
}
