package db

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

var DB *mongo.Database

func InitDB() {
	mongoURI := "mongodb://localhost:27017"

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}

	DB = client.Database("tournament")
	log.Println("Connected to MongoDB!")
}

func AddParticipant(name string) error {
	// Добавляем участника в базу данных
	_, err := DB.Collection("participants").InsertOne(context.Background(), bson.M{"name": name})
	return err
}

func GetAllParticipants() ([]string, error) {
	// Получаем список всех участников из базы данных
	cursor, err := DB.Collection("participants").Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var participants []string
	for cursor.Next(context.Background()) {
		var participant bson.M
		if err := cursor.Decode(&participant); err != nil {
			return nil, err
		}
		participants = append(participants, participant["name"].(string))
	}
	return participants, nil
}

func GetAllParticipantsWithStats() ([]*Participant, error) {
	// Получаем список всех участников из базы данных
	cursor, err := DB.Collection("participants").Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var participants []*Participant
	for cursor.Next(context.Background()) {
		var participant Participant
		if err := cursor.Decode(&participant); err != nil {
			return nil, err
		}
		participants = append(participants, &participant)
	}
	return participants, nil
}

func ParticipantExists(name string) (bool, error) {
	count, err := DB.Collection("participants").CountDocuments(context.Background(), bson.M{"name": name})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func AddTeamCategory(name string, teams []string) error {
	_, err := DB.Collection("team_categories").InsertOne(context.Background(), bson.M{
		"name":  name,
		"teams": teams,
	})
	return err
}

func GetTeamCategories() ([]TeamCategory, error) {
	cursor, err := DB.Collection("team_categories").Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var categories []TeamCategory
	for cursor.Next(context.Background()) {
		var category TeamCategory
		if err := cursor.Decode(&category); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, nil
}

func GetTeamCategoryByName(name string) (*TeamCategory, error) {
	var category TeamCategory
	err := DB.Collection("team_categories").FindOne(context.Background(), bson.M{"name": name}).Decode(&category)
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func SetParticipantTeam(tournamentID int, participantName, team string) error {
	filter := bson.M{
		"id": tournamentID,
	}
	update := bson.M{
		"$set": bson.M{
			"participant_teams." + participantName: team,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err := DB.Collection("tournaments").UpdateOne(context.Background(), filter, update, opts)
	return err
}

func RemoveTeamCategory(name string) error {
	_, err := DB.Collection("team_categories").DeleteOne(context.Background(), bson.M{"name": name})
	return err
}

func IsAdmin(userID int64) (bool, error) {
	filter := bson.M{"user_id": userID}
	count, err := DB.Collection("admins").CountDocuments(context.Background(), filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func AddAdmin(userID int64) error {
	admin := Admin{UserID: userID}
	_, err := DB.Collection("admins").InsertOne(context.Background(), admin)
	return err
}

func RemoveAdmin(userID int64) error {
	filter := bson.M{"user_id": userID}
	_, err := DB.Collection("admins").DeleteOne(context.Background(), filter)
	return err
}
