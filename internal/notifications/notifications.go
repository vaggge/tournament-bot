package notifications

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"net/http"
	"sort"
	"strings"
	"time"
	"tournament-bot/internal/db"
)

const (
	BotToken   = "7012505888:AAEtQoe-AwaoNoC5OPUQaQ6jAqNHKYAKcQk"
	ChannelID  = "@test_bot_botsadfasd"
	TwitchLink = "https://www.twitch.tv/your_channel"
)

func SendTournamentStartMessage(tournament *db.Tournament) error {
	// Формируем список участников с командами
	var participantsWithTeams []string
	for _, participant := range tournament.Participants {
		team := tournament.ParticipantTeams[participant]
		participantWithTeam := fmt.Sprintf("<b>%s</b> (%s)", participant, team)
		participantsWithTeams = append(participantsWithTeams, participantWithTeam)
	}
	participants := strings.Join(participantsWithTeams, "\n")

	// Формируем текст сообщения с использованием HTML
	message := fmt.Sprintf(`
<b>🏆 Новый турнир начался!</b>

<b>Информация о турнире:</b>
<i>Название:</i> %s
<i>Категория:</i> %s

<b>Участники:</b>
%s

<b>🔥 Не пропустите захватывающие матчи турнира!</b>
Смотрите нашу трансляцию на Twitch: <a href="%s">%s</a> 📺

<b>Да начнется битва! ⚽💪</b>
`, tournament.Name, tournament.TeamCategory, participants, TwitchLink, TwitchLink)

	// Создаем MessageConfig для отправки сообщения в канал
	msg := tgbotapi.NewMessageToChannel(ChannelID, message)
	msg.ParseMode = "HTML"

	// Создаем новый экземпляр бота с использованием токена
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		return fmt.Errorf("failed to create bot: %v", err)
	}

	// Устанавливаем время ожидания для бота
	bot.Debug = true
	bot.Client = &http.Client{Timeout: 30 * time.Second}

	// Отправляем сообщение
	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send tournament start message: %v", err)
	}

	return nil
}

func SendMatchResultMessage(tournament *db.Tournament, match *db.Match) error {
	// Формируем текст сообщения с результатами матча
	message := fmt.Sprintf(`
<b>⚽ Результаты матча:</b>
<b>%s</b> %d - %d <b>%s</b>

<b>🏆 Турнирная таблица:</b>
`, match.Team1, match.Score1, match.Score2, match.Team2)

	// Формируем турнирную таблицу
	standings := tournament.Standings
	sort.Slice(standings, func(i, j int) bool {
		if standings[i].Points != standings[j].Points {
			return standings[i].Points > standings[j].Points
		}
		return standings[i].GoalsDifference > standings[j].GoalsDifference
	})

	tableHeader := "<pre>Поз. Команда (Участник)     И   В   Н   П   ГЗ  ГП  РГ  Очки</pre>"
	tableLines := []string{tableHeader}

	for i, standing := range standings {
		team := standing.Team
		played := standing.Played
		won := standing.Won
		drawn := standing.Drawn
		lost := standing.Lost
		goalsFor := standing.GoalsFor
		goalsAgainst := standing.GoalsAgainst
		goalDifference := standing.GoalsDifference
		points := standing.Points

		// Получаем имя участника для текущей команды
		var participant string
		for p, t := range tournament.ParticipantTeams {
			if t == team {
				participant = p
				break
			}
		}

		// Формируем строку с названием команды и именем участника
		teamLine := fmt.Sprintf("<b>%s</b> (%s)", team, participant)

		// Определяем символ для выделения позиции в таблице
		var positionSymbol string
		switch i {
		case 0:
			positionSymbol = "🥇"
		case 1:
			positionSymbol = "🥈"
		case 2:
			positionSymbol = "🥉"
		default:
			positionSymbol = " "
		}

		line := fmt.Sprintf("<pre>%s%2d. %-23s %2d %2d %2d %2d %3d %3d %+3d %4d</pre>", positionSymbol, i+1, teamLine, played, won, drawn, lost, goalsFor, goalsAgainst, goalDifference, points)
		tableLines = append(tableLines, line)
	}

	table := strings.Join(tableLines, "\n")
	message += table

	// Создаем MessageConfig для отправки сообщения в канал
	msg := tgbotapi.NewMessageToChannel(ChannelID, message)
	msg.ParseMode = "HTML"

	// Создаем новый экземпляр бота с использованием токена
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		return fmt.Errorf("failed to create bot: %v", err)
	}

	// Устанавливаем время ожидания для бота
	bot.Debug = true
	bot.Client = &http.Client{Timeout: 30 * time.Second}

	// Отправляем сообщение
	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send match result message: %v", err)
	}

	return nil
}

func SendPlayoffStartMessage(tournament *db.Tournament) error {
	// Формируем текст сообщения о начале плей-офф
	message := fmt.Sprintf("<b>🏆 Начинается плей-офф турнира %s!</b>\n\n", tournament.Name)
	message += "Команды прошли групповой этап и готовы сразиться в захватывающих матчах плей-офф. Кто станет чемпионом? 🤔\n\n"
	message += fmt.Sprintf("Не пропустите ни одного матча! Смотрите нашу трансляцию на Twitch: <a href=\"%s\">%s</a> 📺\n\n", TwitchLink, TwitchLink)
	message += "<b>Сетка плей-офф:</b>\n"

	// Формируем сетку плей-офф
	bracket := "<pre>\n"
	bracket += "Четвертьфинал:\n"
	for _, match := range tournament.Playoff.QuarterFinals {
		bracket += fmt.Sprintf("%s - %s\n", match.Team1, match.Team2)
	}
	bracket += "\nПолуфинал:\n"
	if len(tournament.Playoff.SemiFinals) > 0 {
		bracket += fmt.Sprintf("%s - ?\n", tournament.Playoff.SemiFinals[0].Team1)
	}
	bracket += "\nФинал:\n"
	if tournament.Playoff.Final != nil {
		bracket += fmt.Sprintf("%s - ?\n", tournament.Playoff.Final.Team1)
	} else {
		bracket += "?\n"
	}
	bracket += "</pre>"

	message += bracket

	// Создаем MessageConfig для отправки сообщения в канал
	msg := tgbotapi.NewMessageToChannel(ChannelID, message)
	msg.ParseMode = "HTML"

	// Создаем новый экземпляр бота с использованием токена
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		return fmt.Errorf("failed to create bot: %v", err)
	}

	// Устанавливаем время ожидания для бота
	bot.Debug = true
	bot.Client = &http.Client{Timeout: 30 * time.Second}

	// Отправляем сообщение
	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send playoff start message: %v", err)
	}

	return nil
}

func SendPlayoffMatchResultMessage(tournament *db.Tournament, currentStage string, match *db.Match) error {
	// Формируем текст сообщения с результатами матча плей-офф
	message := fmt.Sprintf("<b>⚽ Результаты матча %s:</b>\n", GetCurrentStageName(currentStage))

	var resultString string
	if match.Penalties {
		resultString = fmt.Sprintf("<b>%s</b> %d:%d (%d:%d) <b>%s</b> (по пенальти)",
			match.Team1, match.Score1, match.Score2,
			match.PenaltyScore1, match.PenaltyScore2, match.Team2)
	} else if match.ExtraTime {
		resultString = fmt.Sprintf("<b>%s</b> %d:%d <b>%s</b> (после овертайма)",
			match.Team1, match.Score1, match.Score2, match.Team2)
	} else {
		resultString = fmt.Sprintf("<b>%s</b> %d:%d <b>%s</b>",
			match.Team1, match.Score1, match.Score2, match.Team2)
	}

	message += resultString + "\n\n"

	// Формируем сетку плей-офф
	message += "<b>🏆 Сетка плей-офф:</b>\n"
	bracket := "<pre>\n"
	bracket += "Четвертьфинал:\n"
	for _, match := range tournament.Playoff.QuarterFinals {
		team1Participant := getParticipantByTeam(tournament.ParticipantTeams, match.Team1)
		team2Participant := getParticipantByTeam(tournament.ParticipantTeams, match.Team2)
		if match.Counted {
			if match.Penalties {
				bracket += fmt.Sprintf("%s (%s) %d:%d (%d:%d) %s (%s) (пен.)\n",
					match.Team1, team1Participant, match.Score1, match.Score2,
					match.PenaltyScore1, match.PenaltyScore2, match.Team2, team2Participant)
			} else if match.ExtraTime {
				bracket += fmt.Sprintf("%s (%s) %d:%d %s (%s) (овертайм)\n",
					match.Team1, team1Participant, match.Score1, match.Score2, match.Team2, team2Participant)
			} else {
				bracket += fmt.Sprintf("%s (%s) %d:%d %s (%s)\n",
					match.Team1, team1Participant, match.Score1, match.Score2, match.Team2, team2Participant)
			}
		} else {
			bracket += fmt.Sprintf("%s (%s) - %s (%s)\n", match.Team1, team1Participant, match.Team2, team2Participant)
		}
	}
	bracket += "\nПолуфинал:\n"
	for _, match := range tournament.Playoff.SemiFinals {
		team1Participant := getParticipantByTeam(tournament.ParticipantTeams, match.Team1)
		team2Participant := getParticipantByTeam(tournament.ParticipantTeams, match.Team2)
		if match.Counted {
			if match.Penalties {
				bracket += fmt.Sprintf("%s (%s) %d:%d (%d:%d) %s (%s) (пен.)\n",
					match.Team1, team1Participant, match.Score1, match.Score2,
					match.PenaltyScore1, match.PenaltyScore2, match.Team2, team2Participant)
			} else if match.ExtraTime {
				bracket += fmt.Sprintf("%s (%s) %d:%d %s (%s) (овертайм)\n",
					match.Team1, team1Participant, match.Score1, match.Score2, match.Team2, team2Participant)
			} else {
				bracket += fmt.Sprintf("%s (%s) %d:%d %s (%s)\n",
					match.Team1, team1Participant, match.Score1, match.Score2, match.Team2, team2Participant)
			}
		} else {
			bracket += fmt.Sprintf("%s (%s) - %s (%s)\n", match.Team1, team1Participant, match.Team2, team2Participant)
		}
	}
	bracket += "\nФинал:\n"
	if tournament.Playoff.Final != nil {
		team1Participant := getParticipantByTeam(tournament.ParticipantTeams, tournament.Playoff.Final.Team1)
		team2Participant := getParticipantByTeam(tournament.ParticipantTeams, tournament.Playoff.Final.Team2)
		if tournament.Playoff.Final.Counted {
			if tournament.Playoff.Final.Penalties {
				bracket += fmt.Sprintf("%s (%s) %d:%d (%d:%d) %s (%s) (пен.)\n",
					tournament.Playoff.Final.Team1, team1Participant,
					tournament.Playoff.Final.Score1, tournament.Playoff.Final.Score2,
					tournament.Playoff.Final.PenaltyScore1, tournament.Playoff.Final.PenaltyScore2,
					tournament.Playoff.Final.Team2, team2Participant)
			} else if tournament.Playoff.Final.ExtraTime {
				bracket += fmt.Sprintf("%s (%s) %d:%d %s (%s) (овертайм)\n",
					tournament.Playoff.Final.Team1, team1Participant,
					tournament.Playoff.Final.Score1, tournament.Playoff.Final.Score2,
					tournament.Playoff.Final.Team2, team2Participant)
			} else {
				bracket += fmt.Sprintf("%s (%s) %d:%d %s (%s)\n",
					tournament.Playoff.Final.Team1, team1Participant,
					tournament.Playoff.Final.Score1, tournament.Playoff.Final.Score2,
					tournament.Playoff.Final.Team2, team2Participant)
			}
		} else {
			bracket += fmt.Sprintf("%s (%s) - %s (%s)\n",
				tournament.Playoff.Final.Team1, team1Participant,
				tournament.Playoff.Final.Team2, team2Participant)
		}
	}
	bracket += "</pre>"

	message += bracket + "\n"

	// Проверяем, есть ли победитель турнира
	if tournament.Playoff.Winner != "" {
		// Получаем имя игрока-победителя
		winnerName := getParticipantByTeam(tournament.ParticipantTeams, tournament.Playoff.Winner)

		// Добавляем сообщение о победителе турнира
		message += fmt.Sprintf("<b>🏆 Победитель турнира: %s 🎉</b>\nПоздравляем <b>%s</b> с победой в турнире!\n", winnerName, winnerName)

		// Добавляем информацию о завершении турнира
		message += "\nТурнир завершен. Спасибо всем участникам и зрителям! 👏\n"
	}

	// Создаем MessageConfig для отправки сообщения в канал
	msg := tgbotapi.NewMessageToChannel(ChannelID, message)
	msg.ParseMode = "HTML"

	// Создаем новый экземпляр бота с использованием токена
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		return fmt.Errorf("failed to create bot: %v", err)
	}

	// Устанавливаем время ожидания для бота
	bot.Debug = true
	bot.Client = &http.Client{Timeout: 30 * time.Second}

	// Отправляем сообщение
	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send playoff match result message: %v", err)
	}

	return nil
}

func getParticipantByTeam(participantTeams map[string]string, teamName string) string {
	for participant, team := range participantTeams {
		if team == teamName {
			return participant
		}
	}
	return "Unknown"
}

func GetCurrentStageName(stage string) string {
	switch stage {
	case "quarter":
		return "четвертьфинала"
	case "semi":
		return "полуфинала"
	case "final":
		return "финала"
	default:
		return ""
	}
}

func SendSeasonRatingMessage() error {
	// Получаем всех участников из базы данных
	participants, err := db.GetAllParticipantsWithStats()
	if err != nil {
		return fmt.Errorf("failed to get participants: %v", err)
	}

	// Сортируем участников по количеству очков и разнице побед и поражений
	sort.Slice(participants, func(i, j int) bool {
		if participants[i].Stats.TotalPoints == participants[j].Stats.TotalPoints {
			return participants[i].Stats.Wins-participants[i].Stats.Losses > participants[j].Stats.Wins-participants[j].Stats.Losses
		}
		return participants[i].Stats.TotalPoints > participants[j].Stats.TotalPoints
	})

	// Формируем текст сообщения с общей таблицей рейтинга сезона
	message := "<b>🏆 Общая таблица рейтинга сезона:</b>\n\n"
	message += "<pre>Поз. Участник               Очки  Турниры  Побед  Ничьих  Пораж.  Голы</pre>\n"
	message += "<pre>------------------------------------------------------------------------------</pre>\n"

	for i, participant := range participants {
		stats := participant.Stats
		line := fmt.Sprintf(
			"<pre>%2d.  %-20s  %4d    %3d     %3d    %3d     %3d    %3d - %3d</pre>\n",
			i+1, participant.Name, stats.TotalPoints, stats.TournamentsPlayed,
			stats.Wins, stats.Draws, stats.Losses, stats.GoalsScored, stats.GoalsConceded,
		)
		message += line
	}

	// Создаем MessageConfig для отправки сообщения в канал
	msg := tgbotapi.NewMessageToChannel(ChannelID, message)
	msg.ParseMode = "HTML"

	// Создаем новый экземпляр бота с использованием токена
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		return fmt.Errorf("failed to create bot: %v", err)
	}

	// Устанавливаем время ожидания для бота
	bot.Debug = true
	bot.Client = &http.Client{Timeout: 30 * time.Second}

	// Отправляем сообщение
	_, err = bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send season rating message: %v", err)
	}

	return nil
}
