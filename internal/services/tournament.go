package services

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
	"tournament-bot/internal/db"
	"tournament-bot/internal/notifications"
)

func GetActiveTournament() (*db.Tournament, error) {
	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), bson.D{{"is_active", true}}).Decode(&tournament)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &tournament, nil
}

func CreateTournament() (*db.Tournament, error) {
	today := time.Now().Format("2006-01-02")
	tournamentName := fmt.Sprintf("%s Tournament #%d", today, getNextTournamentNumber(today))

	tournament := &db.Tournament{
		ID:               getNextTournamentID(),
		Name:             tournamentName,
		Participants:     []string{},
		MinParticipants:  5,
		MaxParticipants:  6,
		ParticipantTeams: make(map[string]string),
		Matches:          []db.Match{},
		Standings:        []db.Standing{},
		IsActive:         false,
		SetupCompleted:   false,
		CreatedAt:        time.Now(),
		IsCompleted:      false,
	}

	_, err := db.DB.Collection("tournaments").InsertOne(context.TODO(), tournament)
	if err != nil {
		return nil, err
	}

	return tournament, nil
}

func EndTournament(tournamentID int) error {
	filter := bson.D{{"id", tournamentID}}
	update := bson.D{{"$set", bson.D{{"is_active", false}}}}
	result, err := db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return errors.New("tournament not found or already ended")
	}
	return nil
}

func DeleteTournament(tournamentID int) error {
	filter := bson.D{{"id", tournamentID}}
	result, err := db.DB.Collection("tournaments").DeleteOne(context.TODO(), filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return errors.New("tournament not found")
	}
	return nil
}

func ToggleParticipant(tournamentID int, participantName string) error {
	filter := bson.M{"id": tournamentID}
	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), filter).Decode(&tournament)
	if err != nil {
		return err
	}

	update := bson.M{}
	if tournament.HasParticipant(participantName) {
		update["$pull"] = bson.M{"participants": participantName}
	} else {
		if len(tournament.Participants) >= tournament.MaxParticipants {
			return fmt.Errorf("tournament has reached the maximum number of participants (%d)", tournament.MaxParticipants)
		}
		update["$addToSet"] = bson.M{"participants": participantName}
	}

	_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	return err
}

func StartTournament(tournamentID int) (*db.Tournament, error) {
	filter := bson.M{"id": tournamentID}
	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), filter).Decode(&tournament)
	if err != nil {
		return nil, err
	}

	if len(tournament.Participants) < tournament.MinParticipants {
		return nil, fmt.Errorf("tournament requires a minimum of %d participants to start", tournament.MinParticipants)
	}

	if len(tournament.Participants) > tournament.MaxParticipants {
		return nil, fmt.Errorf("tournament exceeds the maximum limit of %d participants", tournament.MaxParticipants)
	}

	if tournament.TeamCategory == "" {
		return nil, fmt.Errorf("tournament team category is not set")
	}

	if tournament.IsActive {
		return nil, fmt.Errorf("tournament is already active")
	}

	// Проверяем условия настройки турнира
	setupCompleted := len(tournament.Participants) >= tournament.MinParticipants &&
		len(tournament.Participants) <= tournament.MaxParticipants &&
		tournament.TeamCategory != ""

	update := bson.M{
		"$set": bson.M{
			"is_active":       true,
			"setup_completed": setupCompleted,
		},
	}
	_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return nil, err
	}

	// Получаем обновленный турнир из базы данных
	err = db.DB.Collection("tournaments").FindOne(context.TODO(), filter).Decode(&tournament)
	if err != nil {
		return nil, err
	}

	if !tournament.SetupCompleted {
		return nil, fmt.Errorf("tournament setup is not completed")
	}

	return &tournament, nil
}

func getNextTournamentID() int {
	var lastTournament db.Tournament
	opts := options.FindOne().SetSort(bson.D{{"id", -1}})
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), bson.D{}, opts).Decode(&lastTournament)
	if err != nil {
		return 1
	}
	return lastTournament.ID + 1
}

func GetInactiveTournaments() ([]*db.Tournament, error) {
	filter := bson.M{"$or": []bson.M{
		{"setup_completed": false},
		{"is_active": false},
		{"is_complete": false},
	}}
	cursor, err := db.DB.Collection("tournaments").Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	var tournaments []*db.Tournament
	for cursor.Next(context.TODO()) {
		var tournament db.Tournament
		err := cursor.Decode(&tournament)
		if err != nil {
			return nil, err
		}
		tournaments = append(tournaments, &tournament)
	}

	return tournaments, nil
}

func GetTournament(tournamentID int) (*db.Tournament, error) {
	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), bson.M{"id": tournamentID}).Decode(&tournament)
	if err != nil {
		return nil, err
	}
	return &tournament, nil
}

func SetTournamentTeamCategory(tournamentID int, categoryName string) error {
	filter := bson.M{"id": tournamentID}
	update := bson.M{"$set": bson.M{"team_category": categoryName}}

	_, err := db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	return err
}

func PerformTeamDraw(tournamentID int) (string, error) {
	tournament, err := GetTournament(tournamentID)
	if err != nil {
		return "", err
	}

	category, err := db.GetTeamCategoryByName(tournament.TeamCategory)
	if err != nil {
		return "", err
	}

	// Перемешиваем список команд
	teams := make([]string, len(category.Teams))
	copy(teams, category.Teams)
	rand.Shuffle(len(teams), func(i, j int) {
		teams[i], teams[j] = teams[j], teams[i]
	})

	// Назначаем команды участникам турнира
	var drawResult strings.Builder
	participantTeams := make(map[string]string)
	for i, participant := range tournament.Participants {
		if i >= len(teams) {
			return "", fmt.Errorf("not enough teams for all participants")
		}
		team := teams[i]
		participantTeams[participant] = team
		drawResult.WriteString(fmt.Sprintf("%s - %s\n", participant, team))
	}

	// Создаем записи статистики только для команд участников
	standings := make([]db.Standing, 0)
	for _, team := range participantTeams {
		standing := db.Standing{
			Team:            team,
			Played:          0,
			Won:             0,
			Drawn:           0,
			Lost:            0,
			GoalsFor:        0,
			GoalsAgainst:    0,
			GoalsDifference: 0,
			Points:          0,
		}
		standings = append(standings, standing)
	}

	// Обновляем турнир в базе данных с новыми записями статистики команд и назначенными командами
	filter := bson.M{"id": tournamentID}
	update := bson.M{
		"$set": bson.M{
			"standings":         standings,
			"participant_teams": participantTeams,
		},
	}
	_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return "", err
	}

	return drawResult.String(), nil
}

func GetActiveTournaments() ([]*db.Tournament, error) {
	filter := bson.M{"is_active": true}
	cursor, err := db.DB.Collection("tournaments").Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	var tournaments []*db.Tournament
	for cursor.Next(context.TODO()) {
		var tournament db.Tournament
		err := cursor.Decode(&tournament)
		if err != nil {
			return nil, err
		}
		tournaments = append(tournaments, &tournament)
	}

	return tournaments, nil
}

func getNextTournamentNumber(date string) int {
	filter := bson.M{"date": date}
	update := bson.M{"$inc": bson.M{"count": 1}}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var result struct {
		Count int `bson:"count"`
	}

	err := db.DB.Collection("tournament_counters").FindOneAndUpdate(context.TODO(), filter, update, opts).Decode(&result)
	if err != nil {
		log.Printf("Error getting next tournament number: %v", err)
		return 1
	}

	return result.Count
}

func AddMatchResult(tournamentID int, team1, team2 string, score1, score2 int) error {
	match := db.Match{
		Team1:  team1,
		Team2:  team2,
		Score1: score1,
		Score2: score2,
	}

	filter := bson.M{"id": tournamentID}
	update := bson.M{"$push": bson.M{"matches": match}}

	_, err := db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return err
	}

	// Обновляем турнирную таблицу
	err = updateStandings(tournamentID)
	if err != nil {
		return err
	}

	tournament, err := GetActiveTournament()

	err = notifications.SendMatchResultMessage(tournament, &match)
	if err != nil {
		log.Printf("Error sending match result message: %v", err)
	}

	return nil
}

func updateStandings(tournamentID int) error {
	tournament, err := GetTournament(tournamentID)
	if err != nil {
		return err
	}

	// Создаем карту для быстрого доступа к записям standings по названию команды
	standingsMap := make(map[string]*db.Standing)
	for i := range tournament.Standings {
		standingsMap[tournament.Standings[i].Team] = &tournament.Standings[i]
	}

	for i, match := range tournament.Matches {
		// Проверяем, был ли матч уже учтен при обновлении статистики
		if match.Counted {
			continue
		}

		// Обновляем статистику для каждой команды матча
		standing1 := standingsMap[match.Team1]
		standing2 := standingsMap[match.Team2]

		standing1.Played++
		standing2.Played++

		standing1.GoalsFor += match.Score1
		standing1.GoalsAgainst += match.Score2
		standing2.GoalsFor += match.Score2
		standing2.GoalsAgainst += match.Score1

		standing1.GoalsDifference = standing1.GoalsFor - standing1.GoalsAgainst
		standing2.GoalsDifference = standing2.GoalsFor - standing2.GoalsAgainst

		if match.Score1 > match.Score2 {
			standing1.Won++
			standing1.Points += 3
			standing2.Lost++
		} else if match.Score1 < match.Score2 {
			standing1.Lost++
			standing2.Won++
			standing2.Points += 3
		} else {
			standing1.Drawn++
			standing1.Points++
			standing2.Drawn++
			standing2.Points++
		}

		// Помечаем матч как учтенный

		tournament.Matches[i].Counted = true
	}

	// Сохраняем обновленные standings и matches в базе данных
	filter := bson.M{"id": tournamentID}
	update := bson.M{
		"$set": bson.M{
			"standings": tournament.Standings,
			"matches":   tournament.Matches,
		},
	}
	_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return err
	}

	return nil
}

func DeleteLastMatch(tournamentID int, stageType string) error {
	// Получение турнира из базы данных
	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), bson.M{"id": tournamentID}).Decode(&tournament)
	if err != nil {
		return err
	}

	var deletedMatch *db.Match

	switch stageType {
	case "групповой этап":
		// Проверка наличия матчей в групповом этапе турнира
		if len(tournament.Matches) == 0 {
			return errors.New("no matches found in the group stage of the tournament")
		}

		// Сохранение удаленного матча
		deletedMatch = &tournament.Matches[len(tournament.Matches)-1]

		// Удаление последнего матча из слайса matches
		tournament.Matches = tournament.Matches[:len(tournament.Matches)-1]

		// Обновление турнира в базе данных
		_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), bson.M{"id": tournamentID}, bson.M{"$set": bson.M{"matches": tournament.Matches}})
		if err != nil {
			return err
		}

	case "четвертьфинал":
		// Проверка наличия матчей в четвертьфинале турнира
		if len(tournament.Playoff.QuarterFinals) == 0 {
			return errors.New("no matches found in the quarter-finals of the tournament")
		}

		// Сохранение удаленного матча
		deletedMatch = &tournament.Playoff.QuarterFinals[len(tournament.Playoff.QuarterFinals)-1]

		// Удаление последнего матча из слайса quarter_finals
		tournament.Playoff.QuarterFinals = tournament.Playoff.QuarterFinals[:len(tournament.Playoff.QuarterFinals)-1]

		// Обновление турнира в базе данных
		_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), bson.M{"id": tournamentID}, bson.M{"$set": bson.M{"playoff.quarter_finals": tournament.Playoff.QuarterFinals}})
		if err != nil {
			return err
		}

	case "полуфинал":
		// Проверка наличия матчей в полуфинале турнира
		if len(tournament.Playoff.SemiFinals) == 0 {
			return errors.New("no matches found in the semi-finals of the tournament")
		}

		// Сохранение удаленного матча
		deletedMatch = &tournament.Playoff.SemiFinals[len(tournament.Playoff.SemiFinals)-1]

		// Удаление последнего матча из слайса semi_finals
		tournament.Playoff.SemiFinals = tournament.Playoff.SemiFinals[:len(tournament.Playoff.SemiFinals)-1]

		// Обновление турнира в базе данных
		_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), bson.M{"id": tournamentID}, bson.M{"$set": bson.M{"playoff.semi_finals": tournament.Playoff.SemiFinals}})
		if err != nil {
			return err
		}

	case "финал":
		// Проверка наличия финального матча в турнире
		if tournament.Playoff.Final == nil {
			return errors.New("no final match found in the tournament")
		}

		// Сохранение удаленного матча
		deletedMatch = tournament.Playoff.Final

		// Удаление финального матча
		tournament.Playoff.Final = nil

		// Обновление турнира в базе данных
		_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), bson.M{"id": tournamentID}, bson.M{"$set": bson.M{"playoff.final": nil}})
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid stage type: %s", stageType)
	}

	// Обновление турнирной таблицы
	err = recalculateStandingsAfterDeletion(tournamentID, deletedMatch)
	if err != nil {
		return err
	}

	return nil
}

func recalculateStandingsAfterDeletion(tournamentID int, deletedMatch *db.Match) error {
	// Получение турнира из базы данных
	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), bson.M{"id": tournamentID}).Decode(&tournament)
	if err != nil {
		return err
	}

	// Создание карты для быстрого доступа к записям standings по названию команды
	standingsMap := make(map[string]*db.Standing)
	for i := range tournament.Standings {
		standingsMap[tournament.Standings[i].Team] = &tournament.Standings[i]
	}

	// Обновление статистики для команд удаленного матча
	standing1 := standingsMap[deletedMatch.Team1]
	standing2 := standingsMap[deletedMatch.Team2]

	standing1.Played--
	standing2.Played--

	standing1.GoalsFor -= deletedMatch.Score1
	standing1.GoalsAgainst -= deletedMatch.Score2
	standing2.GoalsFor -= deletedMatch.Score2
	standing2.GoalsAgainst -= deletedMatch.Score1

	standing1.GoalsDifference = standing1.GoalsFor - standing1.GoalsAgainst
	standing2.GoalsDifference = standing2.GoalsFor - standing2.GoalsAgainst

	if deletedMatch.Score1 > deletedMatch.Score2 {
		standing1.Won--
		standing1.Points -= 3
		standing2.Lost--
	} else if deletedMatch.Score1 < deletedMatch.Score2 {
		standing1.Lost--
		standing2.Won--
		standing2.Points -= 3
	} else {
		standing1.Drawn--
		standing1.Points--
		standing2.Drawn--
		standing2.Points--
	}

	// Сохранение обновленных standings в базе данных
	filter := bson.M{"id": tournamentID}
	update := bson.M{"$set": bson.M{"standings": tournament.Standings}}
	_, err = db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return err
	}

	return nil
}

func GetTournamentStandings(tournamentID int) []db.Standing {
	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), bson.M{"id": tournamentID}).Decode(&tournament)
	if err != nil {
		log.Printf("Error getting tournament standings: %v", err)
		return nil
	}
	return tournament.Standings
}

func GetTournamentMatches(tournamentID int) []db.Match {
	var tournament db.Tournament
	err := db.DB.Collection("tournaments").FindOne(context.TODO(), bson.M{"id": tournamentID}).Decode(&tournament)
	if err != nil {
		log.Printf("Error getting tournament matches: %v", err)
		return nil
	}
	return tournament.Matches
}

func UpdateTournament(tournament *db.Tournament) error {
	// Обновляем турнир в базе данных
	filter := bson.M{"id": tournament.ID}
	update := bson.M{"$set": tournament}
	_, err := db.DB.Collection("tournaments").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return err
	}

	return nil
}

func StartPlayoff(tournamentID int) error {
	// Получаем турнир из базы данных
	tournament, err := GetTournament(tournamentID)
	if err != nil {
		return err
	}

	// Проверяем, что групповой этап завершен
	if !tournament.IsActive || !tournament.SetupCompleted {
		return errors.New("group stage is not completed")
	}

	// Сортируем команды по местам в турнирной таблице
	standings := tournament.Standings
	sort.Slice(standings, func(i, j int) bool {
		// Сравнение по количеству очков
		if standings[i].Points != standings[j].Points {
			return standings[i].Points > standings[j].Points
		}
		// Сравнение по разнице забитых и пропущенных мячей
		if standings[i].GoalsDifference != standings[j].GoalsDifference {
			return standings[i].GoalsDifference > standings[j].GoalsDifference
		}
		// Сравнение по количеству забитых мячей
		if standings[i].GoalsFor != standings[j].GoalsFor {
			return standings[i].GoalsFor > standings[j].GoalsFor
		}
		// Сравнение по количеству сыгранных матчей
		if standings[i].Played != standings[j].Played {
			return standings[i].Played > standings[j].Played
		}
		// Сравнение по результатам личных встреч
		headToHeadResult := getHeadToHeadResult(standings[i].Team, standings[j].Team, tournament.Matches)
		if headToHeadResult != 0 {
			return headToHeadResult > 0
		}
		// Сравнение по алфавиту
		return standings[i].Team < standings[j].Team
	})

	// Получаем команды, занявшие соответствующие места
	var teams []string
	for i := 0; i < 4 && i < len(standings); i++ {
		teams = append(teams, standings[i].Team)
	}

	// Создаем структуру плей-офф

	playoff := &db.Playoff{
		CurrentStage: "quarter",
	}

	// Команда, занявшая первое место, выходит в финал
	if len(teams) >= 1 {
		playoff.Final = &db.Match{
			Team1: teams[0],
		}
	}

	// Команда, занявшая второе место, выходит в полуфинал
	if len(teams) >= 2 {
		playoff.SemiFinals = []db.Match{
			{Team1: teams[1]},
		}
	}

	// Команды, занявшие третье и четвертое места, играют в четвертьфинале
	if len(teams) >= 4 {
		playoff.QuarterFinals = []db.Match{
			{Team1: teams[2], Team2: teams[3]},
		}
	}

	// Сохраняем структуру плей-офф в турнире
	tournament.Playoff = playoff

	// Обновляем турнир в базе данных
	err = UpdateTournament(tournament)
	if err != nil {
		return err
	}

	err = notifications.SendPlayoffStartMessage(tournament)
	if err != nil {
		log.Printf("Error sending playoff start message: %v", err)
	}

	return nil
}

func getHeadToHeadResult(team1, team2 string, matches []db.Match) int {
	team1Wins := 0
	team2Wins := 0

	for _, match := range matches {
		if match.Team1 == team1 && match.Team2 == team2 {
			if match.Score1 > match.Score2 {
				team1Wins++
			} else if match.Score1 < match.Score2 {
				team2Wins++
			}
		} else if match.Team1 == team2 && match.Team2 == team1 {
			if match.Score1 > match.Score2 {
				team2Wins++
			} else if match.Score1 < match.Score2 {
				team1Wins++
			}
		}
	}

	if team1Wins > team2Wins {
		return 1
	} else if team1Wins < team2Wins {
		return -1
	} else {
		return 0
	}
}

func AddPlayoffMatch(tournamentID int, team1, team2 string, score1, score2 int, extraTime, penalties bool) (string, error) {
	// Получаем турнир из базы данных
	tournament, err := GetTournament(tournamentID)
	if err != nil {
		return "", err
	}

	currentStage := tournament.Playoff.CurrentStage

	// Проверяем, что турнир не завершен
	if tournament.IsCompleted {
		return "", errors.New("tournament is already completed")
	}

	// Проверяем, что плей-офф начался
	if tournament.Playoff == nil {
		return "", errors.New("playoff has not started")
	}

	// Обновляем текущую стадию плей-офф
	switch tournament.Playoff.CurrentStage {
	case "quarter":
		// Обновляем счет матча четвертьфинала
		if len(tournament.Playoff.QuarterFinals) > 0 {
			tournament.Playoff.QuarterFinals[0].Score1 = score1
			tournament.Playoff.QuarterFinals[0].Score2 = score2
			tournament.Playoff.QuarterFinals[0].ExtraTime = extraTime
			tournament.Playoff.QuarterFinals[0].Penalties = penalties
			tournament.Playoff.QuarterFinals[0].Counted = true

			// Переходим к полуфиналам
			tournament.Playoff.CurrentStage = "semi"
			// Определяем команду-победителя четвертьфинала
			var winner string
			if score1 > score2 {
				winner = team1
			} else {
				winner = team2
			}
			// Добавляем победителя четвертьфинала в полуфинал
			tournament.Playoff.SemiFinals = []db.Match{{Team1: tournament.Playoff.SemiFinals[0].Team1, Team2: winner}}
		}
	case "semi":
		// Обновляем счет матча полуфинала
		if len(tournament.Playoff.SemiFinals) > 0 {
			tournament.Playoff.SemiFinals[0].Score1 = score1
			tournament.Playoff.SemiFinals[0].Score2 = score2
			tournament.Playoff.SemiFinals[0].ExtraTime = extraTime
			tournament.Playoff.SemiFinals[0].Penalties = penalties
			tournament.Playoff.SemiFinals[0].Counted = true

			// Переходим к финалу
			tournament.Playoff.CurrentStage = "final"
			// Определяем команду-победителя полуфинала
			var winner string
			if score1 > score2 {
				winner = team1
			} else {
				winner = team2
			}
			// Добавляем победителя полуфинала в финал
			tournament.Playoff.Final = &db.Match{Team1: tournament.Playoff.Final.Team1, Team2: winner}
		}
	case "final":
		// Обновляем счет финального матча
		tournament.Playoff.Final.Score1 = score1
		tournament.Playoff.Final.Score2 = score2
		tournament.Playoff.Final.ExtraTime = extraTime
		tournament.Playoff.Final.Penalties = penalties
		tournament.Playoff.Final.Counted = true

		// Определяем победителя турнира
		if score1 > score2 {
			tournament.Playoff.Winner = team1
		} else {
			tournament.Playoff.Winner = team2
		}

		tournament.IsCompleted = true
		tournament.IsActive = false

		err = UpdateParticipantStats(tournamentID, tournament)
		if err != nil {
			// Откатываем изменения турнира в случае ошибки
			tournament.IsCompleted = false
			tournament.IsActive = true
			tournament.Playoff.Winner = ""
			updateErr := UpdateTournament(tournament)
			if updateErr != nil {
				log.Printf("failed to rollback tournament update: %v", updateErr)
			}
			return "", fmt.Errorf("failed to update participant stats: %v", err)
		}
		notifications.SendSeasonRatingMessage()
	}

	// Обновляем турнир в базе данных
	err = UpdateTournament(tournament)
	if err != nil {
		return "", err
	}

	return currentStage, nil
}

func getNextStagePairs(matches []db.Match) []db.Match {
	var nextStagePairs []db.Match

	for i := 0; i < len(matches); i += 2 {
		match1 := matches[i]
		var winner1 string
		if match1.Score1 > match1.Score2 {
			winner1 = match1.Team1
		} else {
			winner1 = match1.Team2
		}

		var winner2 string
		if i+1 < len(matches) {
			match2 := matches[i+1]
			if match2.Score1 > match2.Score2 {
				winner2 = match2.Team1
			} else {
				winner2 = match2.Team2
			}
		}

		nextStagePair := db.Match{
			Team1: winner1,
			Team2: winner2,
		}
		nextStagePairs = append(nextStagePairs, nextStagePair)
	}

	return nextStagePairs
}

func allMatchesCounted(matches []db.Match) bool {
	for _, match := range matches {
		if !match.Counted {
			return false
		}
	}
	return true
}

func GetCurrentStageTeams(tournament *db.Tournament) []string {
	if tournament.Playoff == nil {
		return []string{}
	}

	switch tournament.Playoff.CurrentStage {
	case "quarter":
		return getQuarterFinalTeams(tournament)
	case "semi":
		return getSemiFinalTeams(tournament)
	case "final":
		return getFinalTeams(tournament)
	default:
		return []string{}
	}
}

func getWinner(match db.Match) string {
	if match.Score1 > match.Score2 {
		return match.Team1
	} else {
		return match.Team2
	}
}

func getQuarterFinalTeams(tournament *db.Tournament) []string {
	if len(tournament.Playoff.QuarterFinals) >= 1 {
		return []string{tournament.Playoff.QuarterFinals[0].Team1, tournament.Playoff.QuarterFinals[0].Team2}
	}
	return []string{}
}

func getSemiFinalTeams(tournament *db.Tournament) []string {
	if len(tournament.Playoff.SemiFinals) >= 1 {
		return []string{tournament.Playoff.SemiFinals[0].Team1, tournament.Playoff.SemiFinals[0].Team2}
	}
	return []string{}
}

func getFinalTeams(tournament *db.Tournament) []string {
	if tournament.Playoff.Final != nil {
		return []string{tournament.Playoff.Final.Team1, tournament.Playoff.Final.Team2}
	}
	return []string{}
}

func UpdateParticipantStats(tournamentID int, tournament *db.Tournament) error {
	for _, participant := range tournament.Participants {
		var place string
		var points int
		var goalsScored, goalsConceded, wins, losses, draws, matchesPlayed int

		// Получаем статистику участника в групповом этапе турнира
		for _, match := range tournament.Matches {
			if match.Team1 == tournament.ParticipantTeams[participant] || match.Team2 == tournament.ParticipantTeams[participant] {
				matchesPlayed++
				if match.Team1 == tournament.ParticipantTeams[participant] {
					goalsScored += match.Score1
					goalsConceded += match.Score2
					if match.Score1 > match.Score2 {
						wins++
					} else if match.Score1 < match.Score2 {
						losses++
					} else {
						draws++
					}
				} else {
					goalsScored += match.Score2
					goalsConceded += match.Score1
					if match.Score2 > match.Score1 {
						wins++
					} else if match.Score2 < match.Score1 {
						losses++
					} else {
						draws++
					}
				}
			}
		}

		// Получаем статистику участника в матчах плей-офф
		for _, match := range tournament.Playoff.QuarterFinals {
			if match.Team1 == tournament.ParticipantTeams[participant] || match.Team2 == tournament.ParticipantTeams[participant] {
				matchesPlayed++
				if match.Team1 == tournament.ParticipantTeams[participant] {
					goalsScored += match.Score1
					goalsConceded += match.Score2
					if match.Score1 > match.Score2 {
						wins++
					} else {
						losses++
					}
				} else {
					goalsScored += match.Score2
					goalsConceded += match.Score1
					if match.Score2 > match.Score1 {
						wins++
					} else {
						losses++
					}
				}
			}
		}

		for _, match := range tournament.Playoff.SemiFinals {
			if match.Team1 == tournament.ParticipantTeams[participant] || match.Team2 == tournament.ParticipantTeams[participant] {
				matchesPlayed++
				if match.Team1 == tournament.ParticipantTeams[participant] {
					goalsScored += match.Score1
					goalsConceded += match.Score2
					if match.Score1 > match.Score2 {
						wins++
					} else {
						losses++
					}
				} else {
					goalsScored += match.Score2
					goalsConceded += match.Score1
					if match.Score2 > match.Score1 {
						wins++
					} else {
						losses++
					}
				}
			}
		}

		if tournament.Playoff.Final != nil {
			match := *tournament.Playoff.Final
			if match.Team1 == tournament.ParticipantTeams[participant] || match.Team2 == tournament.ParticipantTeams[participant] {
				matchesPlayed++
				if match.Team1 == tournament.ParticipantTeams[participant] {
					goalsScored += match.Score1
					goalsConceded += match.Score2
					if match.Score1 > match.Score2 {
						wins++
					} else {
						losses++
					}
				} else {
					goalsScored += match.Score2
					goalsConceded += match.Score1
					if match.Score2 > match.Score1 {
						wins++
					} else {
						losses++
					}
				}
			}
		}

		// Определяем место участника в плей-офф и начисляем очки
		if tournament.ParticipantTeams[participant] == tournament.Playoff.Winner {
			place = "first"
			points = 8
		} else if participant == getSecondPlace(tournament) {
			place = "second"
			points = 4
		} else if participant == getThirdPlace(tournament) {
			place = "third"
			points = 2
		} else {
			place = "group"
		}

		// Начисляем дополнительные очки за место в групповом этапе
		groupStage := make([]db.Standing, len(tournament.Standings))
		copy(groupStage, tournament.Standings)
		sort.Slice(groupStage, func(i, j int) bool {
			// Сравнение по количеству очков
			if groupStage[i].Points != groupStage[j].Points {
				return groupStage[i].Points > groupStage[j].Points
			}
			// Сравнение по разнице забитых и пропущенных мячей
			if groupStage[i].GoalsDifference != groupStage[j].GoalsDifference {
				return groupStage[i].GoalsDifference > groupStage[j].GoalsDifference
			}
			// Сравнение по количеству забитых мячей
			if groupStage[i].GoalsFor != groupStage[j].GoalsFor {
				return groupStage[i].GoalsFor > groupStage[j].GoalsFor
			}
			// Сравнение по количеству сыгранных матчей
			if groupStage[i].Played != groupStage[j].Played {
				return groupStage[i].Played > groupStage[j].Played
			}
			// Сравнение по результатам личных встреч
			headToHeadResult := getHeadToHeadResult(groupStage[i].Team, groupStage[j].Team, tournament.Matches)
			if headToHeadResult != 0 {
				return headToHeadResult > 0
			}
			// Сравнение по алфавиту
			return groupStage[i].Team < groupStage[j].Team
		})

		for i, standing := range groupStage {
			if standing.Team == tournament.ParticipantTeams[participant] {
				if i < 3 {
					points += 2
				}
				break
			}
		}

		// Обновляем статистику участника в базе данных
		update := bson.M{
			"$inc": bson.M{
				"stats.total_points":       points,
				"stats.goals_scored":       goalsScored,
				"stats.goals_conceded":     goalsConceded,
				"stats.wins":               wins,
				"stats.losses":             losses,
				"stats.draws":              draws,
				"stats.matches_played":     matchesPlayed,
				"stats.tournaments_played": 1,
			},
			"$push": bson.M{
				"stats.tournament_stats": bson.M{
					"tournament_id":  tournamentID,
					"place":          place,
					"points":         points,
					"goals_scored":   goalsScored,
					"goals_conceded": goalsConceded,
					"wins":           wins,
					"losses":         losses,
					"draws":          draws,
					"matches_played": matchesPlayed,
				},
			},
		}

		_, err := db.DB.Collection("participants").UpdateOne(context.TODO(), bson.M{"name": participant}, update)
		if err != nil {
			return err
		}
	}

	return nil
}

func getSecondPlace(tournament *db.Tournament) string {
	if tournament.Playoff.Final != nil {
		if tournament.Playoff.Final.Team1 != tournament.Playoff.Winner {
			return getParticipantByTeam(tournament.ParticipantTeams, tournament.Playoff.Final.Team1)
		} else {
			return getParticipantByTeam(tournament.ParticipantTeams, tournament.Playoff.Final.Team2)
		}
	}
	return ""
}

func getThirdPlace(tournament *db.Tournament) string {
	if len(tournament.Playoff.SemiFinals) > 0 {
		semifinal := tournament.Playoff.SemiFinals[0]
		if semifinal.Team1 != tournament.Playoff.Winner && semifinal.Team1 != getSecondPlace(tournament) {
			return getParticipantByTeam(tournament.ParticipantTeams, semifinal.Team1)
		} else if semifinal.Team2 != tournament.Playoff.Winner && semifinal.Team2 != getSecondPlace(tournament) {
			return getParticipantByTeam(tournament.ParticipantTeams, semifinal.Team2)
		}
	}
	return ""
}

func getParticipantByTeam(participantTeams map[string]string, teamName string) string {
	for participant, team := range participantTeams {
		if team == teamName {
			return participant
		}
	}
	return ""
}
