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

	tableHeader := "<pre>Поз. Команда            И   В   Н   П   ГЗ  ГП  РГ  Очки</pre>"
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

		line := fmt.Sprintf("<pre>%2d.  %-15s %4d %3d %3d %3d %3d %3d %+3d %4d</pre>", i+1, team, played, won, drawn, lost, goalsFor, goalsAgainst, goalDifference, points)
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

func SendPlayoffMatchResultMessage(tournament *db.Tournament, match *db.Match) error {
	// Формируем текст сообщения с результатами матча плей-офф
	message := fmt.Sprintf(`
<b>⚽ Результаты матча плей-офф:</b>
<b>%s</b> %d - %d <b>%s</b>
`, match.Team1, match.Score1, match.Score2, match.Team2)

	// Проверяем текущую стадию плей-офф
	switch tournament.Playoff.CurrentStage {
	case "quarter":
		// Добавляем уведомление о начале четвертьфинала
		message += fmt.Sprintf(`
<b>🏆 Начался четвертьфинал турнира!</b>
Не пропустите захватывающие матчи плей-офф! 🔥
Смотрите нашу трансляцию на Twitch: <a href="%s">%s</a> 📺
`, TwitchLink, TwitchLink)
	case "semi":
		// Добавляем уведомление о начале полуфинала
		message += fmt.Sprintf(`
<b>🏆 Начался полуфинал турнира!</b>
Борьба накаляется! Кто же выйдет в финал? 🤔
Не пропустите решающие матчи на нашей трансляции Twitch: <a href="%s">%s</a> 📺
`, TwitchLink, TwitchLink)
	case "final":
		// Добавляем уведомление о начале финала
		message += fmt.Sprintf(`
<b>🏆 Начался финал турнира!</b>
Кто станет чемпионом? Узнаем совсем скоро! 🥇
Смотрите финальный матч на нашей трансляции Twitch: <a href="%s">%s</a> 📺
`, TwitchLink, TwitchLink)
	}

	// Формируем сетку плей-офф
	bracket := "<pre>\n"
	bracket += "Четвертьфинал:\n"
	for _, match := range tournament.Playoff.QuarterFinals {
		if match.Counted {
			bracket += fmt.Sprintf("%s %d - %d %s\n", match.Team1, match.Score1, match.Score2, match.Team2)
		} else {
			bracket += fmt.Sprintf("%s - %s\n", match.Team1, match.Team2)
		}
	}
	bracket += "\nПолуфинал:\n"
	for _, match := range tournament.Playoff.SemiFinals {
		if match.Counted {
			bracket += fmt.Sprintf("%s %d - %d %s\n", match.Team1, match.Score1, match.Score2, match.Team2)
		} else {
			bracket += fmt.Sprintf("%s - %s\n", match.Team1, match.Team2)
		}
	}
	bracket += "\nФинал:\n"
	if tournament.Playoff.Final != nil {
		if tournament.Playoff.Final.Counted {
			bracket += fmt.Sprintf("%s %d - %d %s\n", tournament.Playoff.Final.Team1, tournament.Playoff.Final.Score1, tournament.Playoff.Final.Score2, tournament.Playoff.Final.Team2)
		} else {
			bracket += fmt.Sprintf("%s - %s\n", tournament.Playoff.Final.Team1, tournament.Playoff.Final.Team2)
		}
	}
	bracket += "</pre>"

	message += fmt.Sprintf(`
<b>🏆 Сетка плей-офф:</b>
%s
`, bracket)

	// Проверяем, есть ли победитель турнира
	if tournament.Playoff.Winner != "" {
		// Получаем имя игрока-победителя
		winnerName := ""
		for player, team := range tournament.ParticipantTeams {
			if team == tournament.Playoff.Winner {
				winnerName = player
				break
			}
		}

		// Добавляем сообщение о победителе турнира
		message += fmt.Sprintf(`
<b>🏆 Победитель турнира: %s 🎉</b>
Поздравляем <b>%s</b> с победой в турнире!
`, winnerName, winnerName)
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
